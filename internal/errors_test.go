package internal

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewAuthError(t *testing.T) {
	err := NewAuthError("msg", errors.New("inner"))
	if err.Message != "msg" || err.Err.Error() != "inner" {
		t.Errorf("unexpected error: %+v", err)
	}
}

func TestNewConfigError(t *testing.T) {
	err := NewConfigError("field", "val", "msg", errors.New("inner"))
	if err.Field != "field" || err.Value != "val" || err.Message != "msg" || err.Err.Error() != "inner" {
		t.Errorf("unexpected error: %+v", err)
	}
}

func TestNewNetworkError(t *testing.T) {
	err := NewNetworkError("op", "url", "msg", errors.New("inner"))
	if err.Operation != "op" || err.URL != "url" || err.Message != "msg" || err.Err.Error() != "inner" {
		t.Errorf("unexpected error: %+v", err)
	}
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("field", "val", "msg", errors.New("inner"))
	if err.Field != "field" || err.Value != "val" || err.Message != "msg" || err.Err.Error() != "inner" {
		t.Errorf("unexpected error: %+v", err)
	}
}

func TestNewProxyError(t *testing.T) {
	err := NewProxyError("op", "msg", errors.New("inner"))
	if err.Operation != "op" || err.Message != "msg" || err.Err.Error() != "inner" {
		t.Errorf("unexpected error: %+v", err)
	}
}

func TestErrorImplementations(t *testing.T) {
	auth := NewAuthError("msg", errors.New("inner"))
	if auth.Error() == "" {
		t.Error("expected non-empty error string")
	}
	if !errors.Is(auth, auth.Err) {
		t.Error("expected errors.Is to match inner error")
	}

	conf := NewConfigError("f", "v", "m", errors.New("inner"))
	if conf.Error() == "" {
		t.Error("expected non-empty error string")
	}
	if !errors.Is(conf, conf.Err) {
		t.Error("expected errors.Is to match inner error")
	}

	neterr := NewNetworkError("op", "url", "m", errors.New("inner"))
	if neterr.Error() == "" {
		t.Error("expected non-empty error string")
	}
	if !errors.Is(neterr, neterr.Err) {
		t.Error("expected errors.Is to match inner error")
	}

	val := NewValidationError("f", "v", "m", errors.New("inner"))
	if val.Error() == "" {
		t.Error("expected non-empty error string")
	}
	if !errors.Is(val, val.Err) {
		t.Error("expected errors.Is to match inner error")
	}

	proxy := NewProxyError("op", "m", errors.New("inner"))
	if proxy.Error() == "" {
		t.Error("expected non-empty error string")
	}
	if !errors.Is(proxy, proxy.Err) {
		t.Error("expected errors.Is to match inner error")
	}
}

func TestWriteHTTPError(t *testing.T) {
	w := &mockResponseWriter{}
	WriteHTTPError(w, http.StatusBadRequest, "bad request")
	if w.status != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.status)
	}
	if w.header.Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json header")
	}
}

func TestWriteHTTPErrorWithDetails(t *testing.T) {
	w := &mockResponseWriter{}
	WriteHTTPErrorWithDetails(w, http.StatusForbidden, "auth", "forbidden", "details")
	if w.status != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.status)
	}
	if w.header.Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json header")
	}
}

func TestWriteAuthenticationError(t *testing.T) {
	w := &mockResponseWriter{}
	WriteAuthenticationError(w)
	if w.status != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.status)
	}
}

func TestWriteAuthorizationError(t *testing.T) {
	w := &mockResponseWriter{}
	WriteAuthorizationError(w)
	if w.status != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.status)
	}
}

func TestWriteValidationError(t *testing.T) {
	w := &mockResponseWriter{}
	WriteValidationError(w, "validation error")
	if w.status != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.status)
	}
}

func TestWriteInternalError(t *testing.T) {
	w := &mockResponseWriter{}
	WriteInternalError(w)
	if w.status != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.status)
	}
}

func TestWriteServiceUnavailableError(t *testing.T) {
	w := &mockResponseWriter{}
	WriteServiceUnavailableError(w)
	if w.status != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.status)
	}
}

func TestWriteRateLimitError(t *testing.T) {
	w := &mockResponseWriter{}
	WriteRateLimitError(w)
	if w.status != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, w.status)
	}
}

func TestErrorTypeChecks(t *testing.T) {
	auth := NewAuthError("msg", nil)
	if !IsAuthenticationError(auth) {
		t.Error("expected IsAuthenticationError true")
	}
	conf := NewConfigError("f", "v", "m", nil)
	if !IsConfigurationError(conf) {
		t.Error("expected IsConfigurationError true")
	}
	neterr := NewNetworkError("op", "url", "m", nil)
	if !IsNetworkError(neterr) {
		t.Error("expected IsNetworkError true")
	}
	val := NewValidationError("f", "v", "m", nil)
	if !IsValidationError(val) {
		t.Error("expected IsValidationError true")
	}
	proxy := NewProxyError("op", "m", nil)
	if !IsProxyError(proxy) {
		t.Error("expected IsProxyError true")
	}
}

type mockResponseWriter struct {
	header http.Header
	status int
}

func (m *mockResponseWriter) Header() http.Header {
	if m.header == nil {
		m.header = make(http.Header)
	}
	return m.header
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.status = statusCode
}
