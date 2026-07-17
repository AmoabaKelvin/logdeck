package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClampTail(t *testing.T) {
	tests := []struct {
		name string
		tail string
		want string
	}{
		{name: "small value passes through", tail: "100", want: "100"},
		{name: "max value passes through", tail: "10000", want: "10000"},
		{name: "over max is clamped", tail: "50000", want: "10000"},
		{name: "all is treated as max", tail: "all", want: "10000"},
		{name: "non-numeric is treated as max", tail: "banana", want: "10000"},
		{name: "negative is treated as max", tail: "-5", want: "10000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampTail(tt.tail); got != tt.want {
				t.Fatalf("clampTail(%q) = %q, want %q", tt.tail, got, tt.want)
			}
		})
	}
}

func TestParseLogOptionsLevel(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/containers/abc/logs/parsed?level=error", nil)
	if got := parseLogOptions(r).Level; got != "error" {
		t.Fatalf("parseLogOptions Level = %q, want %q", got, "error")
	}

	r = httptest.NewRequest("GET", "/api/v1/containers/abc/logs/parsed", nil)
	if got := parseLogOptions(r).Level; got != "" {
		t.Fatalf("parseLogOptions Level = %q, want empty", got)
	}
}

// TestParseLogOptions covers the parameter parsing beyond the level field: the
// booleans, the string passthroughs, and the tail clamp.
func TestParseLogOptions(t *testing.T) {
	r := httptest.NewRequest("GET",
		"/api/v1/containers/abc/logs/parsed?follow=true&timestamps=1&details=true&stdout=false&stderr=true&since=2026-01-01T00:00:00Z&until=2026-01-02T00:00:00Z&search=oops&tail=50", nil)
	opts := parseLogOptions(r)

	if !opts.Follow {
		t.Errorf("Follow = false, want true")
	}
	if !opts.Timestamps {
		t.Errorf("Timestamps = false, want true")
	}
	if !opts.Details {
		t.Errorf("Details = false, want true")
	}
	if opts.ShowStdout {
		t.Errorf("ShowStdout = true, want false")
	}
	if !opts.ShowStderr {
		t.Errorf("ShowStderr = false, want true")
	}
	if opts.Since != "2026-01-01T00:00:00Z" || opts.Until != "2026-01-02T00:00:00Z" {
		t.Errorf("since/until = %q/%q, want the passed values", opts.Since, opts.Until)
	}
	if opts.Search != "oops" {
		t.Errorf("Search = %q, want oops", opts.Search)
	}
	if opts.Tail != "50" {
		t.Errorf("Tail = %q, want 50", opts.Tail)
	}
}

func TestParseLogOptionsTailClamp(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/containers/abc/logs/parsed?tail=999999", nil)
	if got := parseLogOptions(r).Tail; got != "10000" {
		t.Fatalf("over-max tail = %q, want clamped to 10000", got)
	}
}

// TestRunCommandValidation covers the request-body validation in RunCommand,
// which rejects a missing host, an unparseable body, and a blank command
// before any Docker call is attempted.
func TestRunCommandValidation(t *testing.T) {
	ar := &APIRouter{}

	for _, tt := range []struct {
		name string
		host string
		body string
	}{
		{"missing host", "", `{"command":"ls"}`},
		{"invalid json body", "prod", `{`},
		{"blank command", "prod", `{"command":"   "}`},
		{"empty command", "prod", `{"command":""}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/v1/containers/abc/exec/run"
			if tt.host != "" {
				path += "?host=" + tt.host
			}
			w := httptest.NewRecorder()
			ar.RunCommand(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(tt.body)))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

// TestGetContainerLogsParsedInvalidSearch proves an uncompilable search regex
// is rejected with a 400 before the request reaches Docker.
func TestGetContainerLogsParsedInvalidSearch(t *testing.T) {
	ar := &APIRouter{}
	w := httptest.NewRecorder()
	ar.GetContainerLogsParsed(w, httptest.NewRequest(http.MethodGet,
		"/api/v1/containers/abc/logs/parsed?host=prod&search=%5B", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a bad search pattern, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid search pattern") {
		t.Fatalf("expected an invalid-search message, got %q", w.Body.String())
	}
}
