package session

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractIP_xForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18, 150.172.238.178")

	ip := extractIP(r)
	if ip != "203.0.113.50" {
		t.Errorf("expected first IP from X-Forwarded-For, got %q", ip)
	}
}

func TestExtractIP_xForwardedForSingle(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.1")

	ip := extractIP(r)
	if ip != "10.0.0.1" {
		t.Errorf("expected %q, got %q", "10.0.0.1", ip)
	}
}

func TestExtractIP_xForwardedForWithSpaces(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "  192.168.1.1 , 10.0.0.1 ")

	ip := extractIP(r)
	if ip != "192.168.1.1" {
		t.Errorf("expected trimmed IP, got %q", ip)
	}
}

func TestExtractIP_remoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.100:12345"

	ip := extractIP(r)
	if ip != "192.168.1.100" {
		t.Errorf("expected IP from RemoteAddr, got %q", ip)
	}
}

func TestExtractIP_remoteAddrIPv6(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "[::1]:8080"

	ip := extractIP(r)
	if ip != "::1" {
		t.Errorf("expected ::1, got %q", ip)
	}
}

func TestExtractIP_remoteAddrNoPort(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.100"

	// SplitHostPort fails, falls back to raw RemoteAddr.
	ip := extractIP(r)
	if ip != "192.168.1.100" {
		t.Errorf("expected raw RemoteAddr, got %q", ip)
	}
}

func TestExtractIP_empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = ""

	ip := extractIP(r)
	if ip != "" {
		t.Errorf("expected empty, got %q", ip)
	}
}

func TestExtractIP_xForwardedForPriority(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.50")

	ip := extractIP(r)
	if ip != "203.0.113.50" {
		t.Error("X-Forwarded-For should take priority over RemoteAddr")
	}
}

func TestNew_setsMaxAge(t *testing.T) {
	svc := New(nil, 24*time.Hour)
	if svc.maxAge != 24*time.Hour {
		t.Errorf("expected maxAge 24h, got %v", svc.maxAge)
	}
}
