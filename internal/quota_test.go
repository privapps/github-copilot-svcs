package internal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/privapps/github-copilot-svcs/internal"
)

const quotaBody = `{"copilot_plan":"enterprise","quota_reset_date":"2026-10-01","quota_snapshots":{"premium_interactions":{"percent_remaining":73.3,"quota_remaining":19803.3,"unlimited":false,"overage_permitted":true,"credits_used":7200}}}`

func quotaServer(t *testing.T, status int, body string, hits *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			*hits++
		}
		if r.Header.Get("Authorization") != "token gh-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestFetchQuota(t *testing.T) {
	t.Run("parses premium interactions", func(t *testing.T) {
		srv := quotaServer(t, http.StatusOK, quotaBody, nil)
		defer srv.Close()

		q, err := internal.FetchQuota(context.Background(), srv.Client(), srv.URL, "gh-token", "ua")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q.PercentRemaining != 73.3 || q.QuotaRemaining != 19803.3 || q.CreditsUsed != 7200 || !q.OveragePermitted {
			t.Errorf("unexpected quota: %+v", q)
		}
		if q.CopilotPlan != "enterprise" || q.QuotaResetDate != "2026-10-01" {
			t.Errorf("unexpected plan/reset: %+v", q)
		}
	})

	t.Run("requires github token", func(t *testing.T) {
		if _, err := internal.FetchQuota(context.Background(), http.DefaultClient, "http://unused", "", "ua"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("non-200 is an error", func(t *testing.T) {
		srv := quotaServer(t, http.StatusForbidden, `{}`, nil)
		defer srv.Close()
		if _, err := internal.FetchQuota(context.Background(), srv.Client(), srv.URL, "gh-token", "ua"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing premium_interactions is an error", func(t *testing.T) {
		srv := quotaServer(t, http.StatusOK, `{"quota_snapshots":{}}`, nil)
		defer srv.Close()
		if _, err := internal.FetchQuota(context.Background(), srv.Client(), srv.URL, "gh-token", "ua"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestQuotaServiceHandler(t *testing.T) {
	hits := 0
	srv := quotaServer(t, http.StatusOK, quotaBody, &hits)
	defer srv.Close()

	cfg := &internal.Config{GitHubToken: "gh-token"}
	h := internal.NewQuotaService(cfg, srv.Client(), srv.URL).Handler()

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, "/quota", http.NoBody))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"percent_remaining":73.3`) {
			t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
		}
	}
	if hits != 1 {
		t.Errorf("expected cached second call, upstream hits = %d", hits)
	}

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/quota", http.NoBody))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}

	bad := internal.NewQuotaService(&internal.Config{}, srv.Client(), srv.URL).Handler()
	rec = httptest.NewRecorder()
	bad(rec, httptest.NewRequest(http.MethodGet, "/quota", http.NoBody))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 without token, got %d", rec.Code)
	}
}
