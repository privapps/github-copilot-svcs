// Package internal provides Copilot quota lookup logic for github-copilot-svcs.
package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	copilotUserURL = "https://api.github.com/copilot_internal/user"
	quotaTimeout   = 15 * time.Second
	quotaCacheTTL  = 30 * time.Second
	maxQuotaBody   = 1 << 20
)

// QuotaSnapshot describes the premium interactions quota of a Copilot account.
type QuotaSnapshot struct {
	Unlimited         bool    `json:"unlimited"`
	PercentRemaining  float64 `json:"percent_remaining"`
	QuotaRemaining    float64 `json:"quota_remaining"`
	Remaining         float64 `json:"remaining"`
	Entitlement       float64 `json:"entitlement"`
	CreditsUsed       float64 `json:"credits_used"`
	OveragePermitted  bool    `json:"overage_permitted"`
	OverageCount      float64 `json:"overage_count"`
	QuotaResetDate    string  `json:"quota_reset_date,omitempty"`
	CopilotPlan       string  `json:"copilot_plan,omitempty"`
	TokenBasedBilling bool    `json:"token_based_billing"`
}

type copilotUserResponse struct {
	CopilotPlan    string `json:"copilot_plan"`
	QuotaResetDate string `json:"quota_reset_date"`
	QuotaSnapshots struct {
		PremiumInteractions *struct {
			Unlimited         bool    `json:"unlimited"`
			PercentRemaining  float64 `json:"percent_remaining"`
			QuotaRemaining    float64 `json:"quota_remaining"`
			Remaining         float64 `json:"remaining"`
			Entitlement       float64 `json:"entitlement"`
			CreditsUsed       float64 `json:"credits_used"`
			OveragePermitted  bool    `json:"overage_permitted"`
			OverageCount      float64 `json:"overage_count"`
			TokenBasedBilling bool    `json:"token_based_billing"`
		} `json:"premium_interactions"`
	} `json:"quota_snapshots"`
}

// FetchQuota retrieves the premium interactions quota from the given Copilot user endpoint.
func FetchQuota(ctx context.Context, httpClient *http.Client, url, githubToken, userAgent string) (*QuotaSnapshot, error) {
	if githubToken == "" {
		return nil, NewAuthError("no GitHub token available - authentication required", nil)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/json")
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, NewNetworkError("FetchQuota", url, "request failed", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			Warn("Error closing response body", "error", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, NewNetworkError("FetchQuota", url, fmt.Sprintf("HTTP %d response", resp.StatusCode), nil)
	}

	var raw copilotUserResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxQuotaBody)).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode quota response: %w", err)
	}

	p := raw.QuotaSnapshots.PremiumInteractions
	if p == nil {
		return nil, fmt.Errorf("no premium_interactions quota in response")
	}

	return &QuotaSnapshot{
		Unlimited:         p.Unlimited,
		PercentRemaining:  p.PercentRemaining,
		QuotaRemaining:    p.QuotaRemaining,
		Remaining:         p.Remaining,
		Entitlement:       p.Entitlement,
		CreditsUsed:       p.CreditsUsed,
		OveragePermitted:  p.OveragePermitted,
		OverageCount:      p.OverageCount,
		QuotaResetDate:    raw.QuotaResetDate,
		CopilotPlan:       raw.CopilotPlan,
		TokenBasedBilling: p.TokenBasedBilling,
	}, nil
}

// QuotaService serves the quota over HTTP with a short-lived cache.
type QuotaService struct {
	cfg        *Config
	httpClient *http.Client
	url        string

	mu        sync.Mutex
	cached    *QuotaSnapshot
	fetchedAt time.Time
}

// NewQuotaService creates a quota service. An empty url uses the GitHub endpoint.
func NewQuotaService(cfg *Config, httpClient *http.Client, url string) *QuotaService {
	if url == "" {
		url = copilotUserURL
	}
	return &QuotaService{cfg: cfg, httpClient: httpClient, url: url}
}

// Handler returns an HTTP handler for the quota endpoint.
func (s *QuotaService) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		if s.cached == nil || time.Since(s.fetchedAt) > quotaCacheTTL {
			ctx, cancel := context.WithTimeout(r.Context(), quotaTimeout)
			defer cancel()
			q, err := FetchQuota(ctx, s.httpClient, s.url, s.cfg.GitHubToken, s.cfg.Headers.UserAgent)
			if err != nil {
				Error("Failed to fetch quota", "error", err)
				WriteHTTPError(w, http.StatusBadGateway, "failed to fetch quota")
				return
			}
			s.cached, s.fetchedAt = q, time.Now()
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(s.cached); err != nil {
			Warn("Failed to write quota response", "error", err)
		}
	}
}
