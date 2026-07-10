package internal

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// Constants for timeout values
const (
	shutdownTimeout = 10 * time.Second

	// HTTP client configuration
	maxIdleConns        = 100
	maxIdleConnsPerHost = 20
	workerMultiplier    = 2
)

// Server represents the HTTP server and its dependencies
type Server struct {
	config     *Config
	httpServer *http.Server
	httpClient *http.Client
	workerPool *WorkerPool
}

// WorkerPool handles background processing
type WorkerPool struct {
	workers  int
	jobQueue chan func()
	quit     chan bool
	wg       sync.WaitGroup
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int) *WorkerPool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	wp := &WorkerPool{
		workers:  workers,
		jobQueue: make(chan func(), workers*workerMultiplier), // Buffer for burst traffic
		quit:     make(chan bool),
	}

	wp.start()
	return wp
}

func (wp *WorkerPool) start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go func() {
			defer wp.wg.Done()
			for {
				select {
				case job := <-wp.jobQueue:
					job()
				case <-wp.quit:
					return
				}
			}
		}()
	}
}

// Submit adds a job to the worker pool
func (wp *WorkerPool) Submit(job func()) {
	wp.jobQueue <- job
}

// Stop gracefully stops the worker pool
func (wp *WorkerPool) Stop() {
	close(wp.quit)
	wp.wg.Wait()
}

// CreateHTTPClient creates a configured HTTP client
func CreateHTTPClient(cfg *Config) *http.Client {
	return &http.Client{
		Timeout: time.Duration(cfg.Timeouts.HTTPClient) * time.Second,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        maxIdleConns,
			MaxIdleConnsPerHost: maxIdleConnsPerHost,
			IdleConnTimeout:     time.Duration(cfg.Timeouts.IdleConnTimeout) * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   time.Duration(cfg.Timeouts.DialTimeout) * time.Second,
				KeepAlive: time.Duration(cfg.Timeouts.KeepAlive) * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: time.Duration(cfg.Timeouts.TLSHandshake) * time.Second,
		},
	}
}

// NewServer creates a new server instance
func NewServer(cfg *Config, httpClient *http.Client) *Server {
	workerPool := NewWorkerPool(runtime.NumCPU() * workerMultiplier)

	// Create auth service
	authService := NewAuthService(httpClient)

	// Create coalescing cache for models
	coalescingCache := NewCoalescingCache()
	modelsService := NewModelsService(coalescingCache, httpClient)

	// Create proxy service
	proxyService := NewProxyService(cfg, httpClient, authService, workerPool)

	// Create health checker
	healthChecker := NewHealthChecker(httpClient, "dev") // TODO: get version from build

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", modelsService.Handler())
	mux.HandleFunc("/v1/chat/completions", proxyService.Handler())
	mux.HandleFunc("/v1/responses", proxyService.Handler())
	mux.HandleFunc("/health", healthChecker.Handler())

	// Add pprof endpoints for profiling
	mux.HandleFunc("/debug/pprof/", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/cmdline", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/profile", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/symbol", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/trace", http.DefaultServeMux.ServeHTTP)

	port := cfg.Port
	if port == 0 {
		port = 8081 // default port
	}

	// Build middleware chain
	var handler http.Handler = mux

	// Apply middleware in reverse order (last applied = first executed)
	handler = SecurityHeadersMiddleware(handler)
	handler = CORSMiddleware(cfg)(handler)
	handler = LoggingMiddleware(handler)
	handler = RecoveryMiddleware(handler)
	// Note: TimeoutMiddleware could be added here if needed per-request timeouts
	// handler = TimeoutMiddleware(time.Duration(cfg.Timeouts.ProxyContext) * time.Second)(handler)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  time.Duration(cfg.Timeouts.ServerRead) * time.Second,
		WriteTimeout: time.Duration(cfg.Timeouts.ServerWrite) * time.Second,
		IdleTimeout:  time.Duration(cfg.Timeouts.ServerIdle) * time.Second,
	}

	return &Server{
		config:     cfg,
		httpServer: httpServer,
		httpClient: httpClient,
		workerPool: workerPool,
	}
}

// Start starts the HTTP server with graceful shutdown
func (s *Server) Start() error {
	s.setupGracefulShutdown()

	port := s.config.Port
	if port == 0 {
		port = 8081
	}

	fmt.Printf("Starting GitHub Copilot proxy server on port %d...\n", port)
	fmt.Printf("Endpoints:\n")
	fmt.Printf("  - Models: http://localhost:%d/v1/models\n", port)
	fmt.Printf("  - Chat: http://localhost:%d/v1/chat/completions\n", port)
	fmt.Printf("  - Responses: http://localhost:%d/v1/responses\n", port)
	fmt.Printf("  - Health: http://localhost:%d/health\n", port)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %v", err)
	}

	return nil
}

// Stop gracefully stops the server
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	fmt.Println("Stopping worker pool...")
	s.workerPool.Stop()
	fmt.Println("Worker pool stopped.")

	fmt.Println("Shutting down HTTP server...")
	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		fmt.Printf("Error during HTTP server shutdown: %v\n", err)
		return err
	}
	fmt.Println("HTTP server shutdown complete.")

	return nil
}

func (s *Server) setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		fmt.Println("\nGracefully shutting down...")

		if err := s.Stop(); err != nil {
			Error("Server shutdown error", "error", err)
		}
	}()
}

// healthHandler is now replaced by the comprehensive HealthChecker
