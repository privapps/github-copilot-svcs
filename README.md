# GitHub Copilot Service Proxy

This project provides a reverse proxy for GitHub Copilot, exposing OpenAI-compatible endpoints for use with tools and clients that expect the OpenAI API. It follows the authentication and token management approach used by [OpenCode](https://github.com/sst/opencode).

> **❗ IMPORTANT: Vision/Image capability is not available on all GitHub Copilot accounts!**
> - Even if your account supports GPT-4o, **vision features may not be enabled** for your Copilot subscription.
> - Some orgs and accounts do not have access to vision/image generation/analysis, or may have different levels of support than OpenAI direct accounts.
> - This proxy supports vision for Copilot accounts _if_ your Copilot subscription supports it. If your account does not, you will receive a relevant error from the upstream API.
> - For latest details, check your Copilot subscription or contact GitHub support for your organization/account.

## Features

- **OAuth Device Flow Authentication**: Secure authentication with GitHub Copilot using the same flow as OpenCode
- **Vision Support**: Full support for image/vision requests with base64-encoded images in OpenAI-compatible format
- **Advanced Token Management**: 
  - Proactive token refresh (refreshes at 20% of token lifetime, minimum 5 minutes)
  - Exponential backoff retry logic for failed token refreshes
  - Automatic fallback to full re-authentication when needed
  - Detailed token status monitoring
- **Robust Request Handling**:
  - Automatic retry with exponential backoff for chat completions (3 attempts)
  - Network error recovery and rate limiting handling
  - 30-second request timeout protection
- **OpenAI-Compatible API**: Exposes `/v1/chat/completions` and `/v1/models` endpoints
- **Request/Response Transformation**: Handles model name mapping and ensures OpenAI compatibility
- **Configurable Port**: Default port 8081, configurable via CLI or config file
- **Health Monitoring**: `/health` endpoint for service monitoring
- **Graceful Shutdown**: Proper signal handling and graceful server shutdown
- **Comprehensive Logging**: Request/response logging for debugging and monitoring
- **Enhanced CLI Commands**: Status monitoring, manual token refresh, and detailed configuration display
- **Production-Ready Performance**: HTTP connection pooling, circuit breaker, request coalescing, and memory optimization
- **Monitoring & Profiling**: Built-in pprof endpoints for memory, CPU, and goroutine analysis

## Downloads

Pre-built binaries are available for each release on the [Releases page](https://github.com/privapps/github-copilot-svcs/releases).

Available platforms:
- **Linux**: AMD64, ARM64
- **macOS**: AMD64 (Intel), ARM64 (Apple Silicon) 
- **Windows**: AMD64, ARM64

### Docker Images

Docker images are automatically built and published to GitHub Container Registry via GitHub Actions:

```bash
# Pull the latest image
docker pull ghcr.io/privapps/github-copilot-svcs:latest

# Pull a specific version (example)
docker pull ghcr.io/privapps/github-copilot-svcs:0.0.2
```

Available architectures:
- `linux/amd64`
- `linux/arm64`

### Automated Releases

Releases are automatically created when code is merged to the `main` branch:
- Version numbers follow semantic versioning (starting from v0.0.1)
- Cross-platform binaries are built and attached to each release
- Release notes include download links for all supported platforms

## Performance & Production Features

This service includes enterprise-grade performance optimizations:

### 🚀 HTTP Server Optimizations
- **Connection Pooling**: Shared HTTP client with configurable connection limits (100 max idle, 20 per host)
- **Configurable Timeouts**: Fully customizable timeout settings via `config.json` for all server operations
- **Streaming Support**: Read (30s), Write (300s), and Idle (120s) timeouts optimized for AI chat streaming
- **Long Response Handling**: HTTP client and proxy context timeouts support up to 300s (5 minutes) for extended AI conversations
- **Request Limits**: 5MB request body size limit to prevent memory exhaustion
- **Advanced Transport**: Configurable dial timeout (10s), TLS handshake timeout (10s), keep-alive (30s)

### 🔄 Reliability & Concurrency
- **Circuit Breaker**: Automatic failure detection and recovery (5 failure threshold, 30s timeout)
- **Context Propagation**: Request contexts with 25s timeout and proper cancellation
- **Request Coalescing**: Deduplicates identical concurrent requests to models endpoint
- **Exponential Backoff**: Enhanced retry logic with circuit breaker integration
- **Worker Pool**: Concurrent request processing with dedicated worker goroutines (CPU*2 workers)

### 💾 Resource Management
- **Buffer Pooling**: sync.Pool for request/response buffer reuse to reduce GC pressure
- **Memory Optimization**: Streaming support with 32KB buffers for large responses
- **Graceful Shutdown**: Proper resource cleanup and coordinated shutdown with worker pool termination
- **Shared Clients**: Centralized HTTP client eliminates resource duplication
- **Worker Pool Management**: Automatic worker lifecycle management with graceful termination

### 📊 Monitoring & Observability
- **Profiling Endpoints**: `/debug/pprof/*` for memory, CPU, and goroutine analysis
- **Enhanced Logging**: Circuit breaker state, request coalescing, and performance data
- **Health Monitoring**: Detailed `/health` endpoint for load balancer integration

## Quickstart with Makefile

If you have `make` installed, you can build, run, and test the project easily:

```bash
make build      # Build the binary
make run        # Start the proxy server
make test       # Run unit tests
make test-all   # Run all tests
make test-coverage # Generate coverage report
make clean      # Remove the binary
make lint       # Run linting
make fmt        # Format code
make vet        # Run go vet
make security   # Run security analysis
make docker-build       # Build Docker image
make docker-run         # Run Docker container
```

## Filtering Allowed Models

You can control which models are available by specifying `allowed_models` in your config file (`config.json`).

Example:
```json
{
  "allowed_models": ["gpt-4o", "claude-3.7-sonnet"]
}
```

- If set, both CLI and REST /v1/models lists are filtered and show a note.
- Proxy requests to /v1/chat/completions will only allow those models, rejecting others with HTTP 400.
- If omitted or set to null, all models are permitted (default behavior).

## Building for Different OS/Architectures

You can build binaries for different platforms using the following Makefile targets:

- `make build-linux-amd64`      Build for Linux amd64
- `make build-linux-arm64`      Build for Linux arm64
- `make build-darwin-amd64`     Build for macOS amd64
- `make build-darwin-arm64`     Build for macOS arm64
- `make build-windows-amd64`    Build for Windows amd64
- `make build-windows-arm64`    Build for Windows arm64

The output binaries will be named accordingly (e.g., `github-copilot-svcs-windows-arm64.exe`).

## Installation & Usage

### 1. Build the Application
```bash
make build
# or manually:
go build -o github-copilot-svcs
```

### 2. Optional: Configure Timeouts
```bash
# Copy example config and customize timeout values
cp config.example.json ~/.local/share/github-copilot-svcs/config.json
# Edit the timeouts section as needed
```

### 3. First Time Setup & Authentication
```bash
./github-copilot-svcs auth
```

### 4. Start the Proxy Server
```bash
make run
# or manually:
./github-copilot-svcs run
```

## Docker Deployment
```
docker run --rm \
  -p 8081:8081 \
  -v ~/.local/share/github-copilot-svcs:/home/appuser/.local/share/github-copilot-svcs  \
  ghcr.io/privapps/github-copilot-svcs:latest
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `run`   | Run the proxy server (default command) |
| `auth`   | Authenticate with GitHub Copilot using device flow |
| `status` | Show detailed authentication and token status |
| `config` | Display current configuration details |
| `models` | List all available AI models |
| `refresh`| Manually force token refresh |
| `version`| Show version information |
| `help`   | Show usage information |

### Enhanced Status Monitoring

The `status` command now provides detailed token information with optional JSON output:

```bash
./github-copilot-svcs status
./github-copilot-svcs status --json  # JSON format output
```

Example output:
```
Configuration file: ~/.local/share/github-copilot-svcs/config.json
Port: 8081
Authentication: ✓ Authenticated
Token expires: in 29m 53s (1793 seconds)
Status: ✅ Token is healthy
Has GitHub token: true
Refresh interval: 1500 seconds
```

Status indicators:
- ✅ **Token is healthy**: Token has plenty of time remaining
- ⚠️ **Token will be refreshed soon**: Token is approaching refresh threshold
- ❌ **Token needs refresh**: Token has expired or will expire very soon

## API Endpoints

Once running, the proxy exposes these OpenAI-compatible endpoints:

### Chat Completions
```bash
POST http://localhost:8081/v1/chat/completions
Content-Type: application/json

{
  "model": "gpt-4",
  "messages": [
    {"role": "user", "content": "Hello, world!"}
  ],
  "max_tokens": 100
}
```

### Completions
This endpoint is OpenAI-compatible and proxies requests to the upstream Copilot API `/completions` endpoint.

```bash
POST http://localhost:8081/v1/completions
Content-Type: application/json

{
  "model": "gpt-4",
  "prompt": "Write a hello world in Python",
  "max_tokens": 100
}
```

### Available Models
```bash
GET http://localhost:8081/v1/models
```

### Health Check
```bash
GET http://localhost:8081/health
```

### Profiling Endpoints (Production Monitoring)
```bash
GET http://localhost:8081/debug/pprof/          # Overview of available profiles
GET http://localhost:8081/debug/pprof/heap      # Memory heap profile
GET http://localhost:8081/debug/pprof/goroutine # Goroutine profile
GET http://localhost:8081/debug/pprof/profile   # CPU profile (30s sampling)
GET http://localhost:8081/debug/pprof/trace     # Execution trace
```

## Reliability & Error Handling

### Automatic Token Management

The proxy implements proactive token management to minimize authentication interruptions:

- **Proactive Refresh**: Tokens are refreshed when 20% of their lifetime remains (typically 5-6 minutes before expiration for 25-minute tokens)
- **Retry Logic**: Failed token refreshes are retried up to 3 times with exponential backoff (2s, 8s, 18s delays)
- **Fallback Authentication**: If token refresh fails completely, the system falls back to full device flow re-authentication
- **Background Monitoring**: Token status is continuously monitored during API requests

### Request Retry Logic

Chat completion requests are automatically retried to handle transient failures:

- **Automatic Retries**: Up to 3 attempts for failed requests
- **Smart Retry Logic**: Only retries on network errors, server errors (5xx), rate limiting (429), and timeouts (408)
- **Exponential Backoff**: Retry delays of 1s, 4s, 9s to avoid overwhelming the API
- **Timeout Protection**: 30-second timeout per request attempt

### Error Recovery

```bash
# Manual token refresh if needed
./github-copilot-svcs refresh

# Check current token status
./github-copilot-svcs status

# Re-authenticate if all else fails
./github-copilot-svcs auth
```

## Configuration


The configuration is stored in `~/.local/share/github-copilot-svcs/config.json`:

```json
{
  "port": 8081,
  "github_token": "gho_...",
  "copilot_token": "ghu_...",
  "expires_at": 1720000000,
  "refresh_in": 1500,
  "headers": {
    "user_agent": "GitHubCopilotChat/0.29.1",
    "editor_version": "vscode/1.102.3",
    "editor_plugin_version": "copilot-chat/0.29.1",
    "copilot_integration_id": "vscode-chat",
    "openai_intent": "conversation-edits",
    "x_initiator": "user"
  },
  "timeouts": {
    "http_client": 300,
    "server_read": 30,
    "server_write": 300,
    "server_idle": 120,
    "proxy_context": 300,
    "circuit_breaker": 30,
    "keep_alive": 30,
    "tls_handshake": 10,
    "dial_timeout": 10,
    "idle_conn_timeout": 90
  }
}
```


### Configuration Fields

- `port`: Server port (default: 8081)
- `github_token`: GitHub OAuth token for Copilot access
- `copilot_token`: GitHub Copilot API token
- `expires_at`: Unix timestamp when the Copilot token expires
- `refresh_in`: Seconds until token should be refreshed (typically 1500 = 25 minutes)
- `headers`: (optional) HTTP headers to use for all Copilot API requests (see below)
### HTTP Headers Configuration

The `headers` section allows you to customize the HTTP headers sent to the Copilot API. All fields are optional; defaults are shown below:

| Field                    | Default Value                  | Description                                      |
|--------------------------|-------------------------------|--------------------------------------------------|
| `user_agent`             | GitHubCopilotChat/0.29.1      | User-Agent header for all requests                |
| `editor_version`         | vscode/1.102.3                | Editor-Version header                             |
| `editor_plugin_version`  | copilot-chat/0.29.1           | Editor-Plugin-Version header                      |
| `copilot_integration_id` | vscode-chat                   | Copilot-Integration-Id header                     |
| `openai_intent`          | conversation-edits             | Openai-Intent header                              |
| `x_initiator`            | user                           | X-Initiator header                                |

You can override any of these by editing your `config.json`.

### Timeout Configuration

All timeout values are specified in seconds and have sensible defaults:

| Field | Default | Description |
|-------|---------|-------------|
| `http_client` | 300 | HTTP client timeout for outbound requests to GitHub Copilot API |
| `server_read` | 30 | Server timeout for reading incoming requests |
| `server_write` | 300 | Server timeout for writing responses (increased for streaming) |
| `server_idle` | 120 | Server timeout for idle connections |
| `proxy_context` | 300 | Request context timeout for proxy operations |
| `circuit_breaker` | 30 | Circuit breaker recovery timeout when API is failing |
| `keep_alive` | 30 | TCP keep-alive timeout for HTTP connections |
| `tls_handshake` | 10 | TLS handshake timeout |
| `dial_timeout` | 10 | Connection dial timeout |
| `idle_conn_timeout` | 90 | Idle connection timeout in connection pool |

**Streaming Support**: The service is optimized for long-running streaming chat completions with timeouts up to 300 seconds (5 minutes) to support extended AI conversations.

**Custom Configuration**: You can copy `config.example.json` as a starting point and modify timeout values based on your environment:

```bash
cp config.example.json ~/.local/share/github-copilot-svcs/config.json
# Edit the timeouts section as needed
```

## Authentication Flow

The authentication follows GitHub Copilot's OAuth device flow:

1. **Device Authorization**: Generates a device code and user code
2. **User Authorization**: User visits GitHub and enters the user code
3. **Token Exchange**: Polls for GitHub OAuth token
4. **Copilot Token**: Exchanges GitHub token for Copilot API token
5. **Automatic Refresh**: Refreshes Copilot token as needed

## Model Mapping

The proxy automatically maps common model names to GitHub Copilot models:

| Input Model | GitHub Copilot Model | Provider |
|-------------|---------------------|----------|
| `gpt-4o`, `gpt-4.1`, `gpt-5` | As specified | OpenAI |
| `o3`, `o3-mini`, `o4-mini` | As specified | OpenAI |
| `claude-3.5-sonnet`, `claude-3.7-sonnet`, `claude-3.7-sonnet-thought` | As specified | Anthropic |
| `claude-opus-4`, `claude-sonnet-4` | As specified | Anthropic |
| `gemini-2.5-pro`, `gemini-2.0-flash-001` | As specified | Google |

**Supported Model Categories:**
- **OpenAI GPT Models**: GPT-4o, GPT-4.1, O3/O4 reasoning models
- **Anthropic Claude Models**: Claude 3.5/3.7 Sonnet variants, Claude Opus/Sonnet 4
- **Google Gemini Models**: Gemini 2.0/2.5 Pro and Flash models
- There are **additional models** available for use. For more information and details about these models, please refer to your GitHub Copilot subscription page.

## Security

- Tokens are stored securely in the user's home directory with restricted permissions (0700)
- All communication with GitHub Copilot uses HTTPS
- No sensitive data is logged
- Automatic token refresh prevents long-lived token exposure

## Troubleshooting

### Authentication Issues
```bash
# Re-authenticate
./github-copilot-svcs auth

# Check current status
./github-copilot-svcs status
```

### Connection Issues
```bash
# Check if service is running
curl http://localhost:8081/health

# View logs (if running in foreground)
./github-copilot-svcs run
```

### Port Conflicts
```bash
# Use a different port
# Edit ~/.local/share/github-copilot-svcs/config.json
# Or delete config file and restart to select new port
```

## Integration Examples

### Using with curl
```bash
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Write a hello world in Python"}],
    "max_tokens": 100
  }'
```

### Vision/Image Requests

The proxy fully supports vision capabilities with base64-encoded images in OpenAI-compatible format:

```bash
# Example with base64-encoded image
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "What is in this image?"},
        {"type": "image_url", "image_url": {"url": "data:image/jpeg;base64,/9j/4AAQ..."}}
      ]
    }],
    "max_tokens": 300
  }'
```

**Vision Features:**
- Supports multi-part message content (text + images)
- Accepts base64-encoded images as data URIs
- Supports `detail` parameter (`auto`, `low`, `high`)
- Compatible with vision-capable models (gpt-4o, gpt-4-vision, etc.)
- Backward compatible with text-only requests

**Example Script:**
The repository includes `test_vision_proxy.sh` that demonstrates vision capabilities:
```bash
./test_vision_proxy.sh dog.jpeg "Describe this image in detail"
```

### Using with OpenAI Python Client
```python
import openai

# Point OpenAI client to the proxy
client = openai.OpenAI(
    base_url="http://localhost:8081/v1",
    api_key="dummy"  # Not used, but required by client
)

response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello, world!"}]
)
print(response.choices[0].message.content)
```

### Using with LangChain
```python
from langchain.llms import OpenAI

llm = OpenAI(
    openai_api_base="http://localhost:8081/v1",
    openai_api_key="dummy"  # Not used
)
response = llm("Write a hello world in Python")
print(response)
```

## Development

### Building from Source
```bash
git clone <repository>
cd github-copilot-svcs
make build
# or manually:
go mod tidy
go build -o github-copilot-svcs ./cmd/github-copilot-svcs
```

### Running Tests
```bash
make test          # Run unit tests
make test-all      # Run all tests (unit + integration)
make test-coverage # Run tests with coverage report
# or manually:
go test ./test/...
```

### Test Coverage
The project includes comprehensive test coverage:
- **Unit Tests**: Testing individual components (auth, config, logger)
- **Integration Tests**: Testing API endpoints and server functionality  
- **Coverage Reports**: HTML and terminal coverage reports available

Generate coverage reports:
```bash
make test-coverage  # Generates coverage.html and shows terminal summary
```

Current test coverage: **~45%** across all packages, with excellent coverage in core components like logging (95%+) and configuration (58%+).

## License

Apache License 2.0 - see LICENSE file for details.

This is free software: you are free to change and redistribute it under the terms of the Apache 2.0 license.

## Contributing

We welcome contributions! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch (use descriptive names)
3. Make your changes (follow Go code style and best practices)
4. Add or update tests as needed
5. Run all tests and ensure coverage is not reduced
6. Document your changes in the README if relevant
7. Submit a pull request with a clear description

## Security

- Tokens and secrets are stored securely in the user's home directory with restricted permissions (0700)
- No sensitive data is logged
- All communication with GitHub Copilot uses HTTPS
- Automatic token refresh prevents long-lived token exposure
- Do not commit secrets or sensitive config files; check your `.gitignore`
- For security issues, please contact the maintainers directly

## FAQ / Common Issues

**Q: Authentication fails or times out**
A: Run `./github-copilot-svcs auth` again and check your network connection. Ensure your GitHub account has Copilot access.

**Q: Service won't start or port is in use**
A: Edit your config file to use a different port, or stop the conflicting service.

**Q: Token expires too quickly**
A: Check your system clock and ensure the refresh interval in config is set correctly.

**Q: How do I update configuration?**
A: Edit `~/.local/share/github-copilot-svcs/config.json` or use environment variables if supported.

**Q: How do I report a bug or request a feature?**
A: Open an issue on GitHub with details about your environment and the problem.

## Support

For issues and questions:
1. Check the troubleshooting and FAQ sections
2. Review the logs for error messages
3. Open an issue with detailed information about your setup and the problem
