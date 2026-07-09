package api

import (
	"net/http/httptest"
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
