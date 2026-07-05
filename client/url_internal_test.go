package client

import (
	"net/url"
	"strings"
	"testing"
)

func TestEngineWSURL(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantScheme string
		wantPath   string
		wantErr    bool
	}{
		{name: "http becomes ws", in: "http://host:3000", wantScheme: "ws", wantPath: "/socket.io/"},
		{name: "https becomes wss", in: "https://host", wantScheme: "wss", wantPath: "/socket.io/"},
		{name: "ws stays ws", in: "ws://host", wantScheme: "ws", wantPath: "/socket.io/"},
		{name: "wss stays wss", in: "wss://host", wantScheme: "wss", wantPath: "/socket.io/"},
		{name: "unknown scheme defaults to ws", in: "foo://host", wantScheme: "ws", wantPath: "/socket.io/"},
		{name: "custom path keeps and gets trailing slash", in: "http://host/base", wantScheme: "ws", wantPath: "/base/"},
		{name: "path with trailing slash preserved", in: "http://host/base/", wantScheme: "ws", wantPath: "/base/"},
		{name: "malformed url errors", in: "http://[::1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engineWSURL(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			u, err := url.Parse(got)
			if err != nil {
				t.Fatalf("result is not a valid URL: %v", err)
			}
			if u.Scheme != tt.wantScheme {
				t.Fatalf("scheme = %q, want %q", u.Scheme, tt.wantScheme)
			}
			if u.Path != tt.wantPath {
				t.Fatalf("path = %q, want %q", u.Path, tt.wantPath)
			}
			q := u.Query()
			if q.Get("EIO") != "4" || q.Get("transport") != "websocket" {
				t.Fatalf("query missing EIO/transport: %q", u.RawQuery)
			}
			if !strings.HasPrefix(got, tt.wantScheme+"://") {
				t.Fatalf("url %q does not start with %s://", got, tt.wantScheme)
			}
		})
	}
}
