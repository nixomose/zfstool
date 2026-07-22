package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSmartRejectsPathTraversal(t *testing.T) {
	s := NewServer()
	cases := []string{
		"/v1/disk/..%2F..%2Fetc%2Fpasswd/smart",
		"/v1/disk/%2Fdev%2F..%2F..%2Fetc%2Fpasswd/smart",
	}
	for _, path := range cases {
		req := httptest.NewRequest("GET", path, nil)
		rr := httptest.NewRecorder()
		s.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%s: got %d body=%s", path, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "invalid") && !strings.Contains(rr.Body.String(), "/dev") {
			t.Fatalf("%s: unexpected body %s", path, rr.Body.String())
		}
	}
}

func TestSmartAllowsShortName(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest("GET", "/v1/disk/sda/smart", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	// 200 with smartctl error is fine (no disk); must not be 400 validation failure.
	if rr.Code == http.StatusBadRequest {
		t.Fatalf("short name rejected: %s", rr.Body.String())
	}
}

func TestPoolStatusRejectsFlagInjection(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest("GET", "/v1/pools/-c,iostat-10s/status", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got %d %s", rr.Code, rr.Body.String())
	}
}
