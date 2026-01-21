package api

import (
	"fmt"
	"net"
	"testing"
)

func TestParseAddr(t *testing.T) {
	t.Parallel()
	tests := []struct {
		addr     string
		wantHost string
		wantPort int
		wantErr  bool
	}{
		{":8080", "", 8080, false},
		{":3000", "", 3000, false},
		{"127.0.0.1:8080", "127.0.0.1", 8080, false},
		{"localhost:9000", "localhost", 9000, false},
		{"invalid", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			host, port, err := parseAddr(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAddr(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
				return
			}
			if host != tt.wantHost {
				t.Errorf("parseAddr(%q) host = %q, want %q", tt.addr, host, tt.wantHost)
			}
			if port != tt.wantPort {
				t.Errorf("parseAddr(%q) port = %d, want %d", tt.addr, port, tt.wantPort)
			}
		})
	}
}

func TestFindAvailablePort(t *testing.T) {
	t.Parallel()
	// Test finding a port
	ln, port, err := findAvailablePort("", 18080, 10)
	if err != nil {
		t.Fatalf("findAvailablePort failed: %v", err)
	}
	defer func() { _ = ln.Close() }()

	if port < 18080 || port >= 18090 {
		t.Errorf("port = %d, want in range [18080, 18090)", port)
	}

	// Verify the listener is actually bound
	_, err = net.Listen("tcp", ln.Addr().String())
	if err == nil {
		t.Error("expected port to be in use")
	}
}

func TestFindAvailablePort_SkipsBusy(t *testing.T) {
	t.Parallel()
	// Occupy first port
	ln1, err := net.Listen("tcp", ":19080")
	if err != nil {
		t.Fatalf("failed to occupy port: %v", err)
	}
	defer func() { _ = ln1.Close() }()

	// Should skip to next port
	ln2, port, err := findAvailablePort("", 19080, 10)
	if err != nil {
		t.Fatalf("findAvailablePort failed: %v", err)
	}
	defer func() { _ = ln2.Close() }()

	if port != 19081 {
		t.Errorf("port = %d, want 19081 (should skip busy 19080)", port)
	}
}

func TestFindAvailablePort_AllBusy(t *testing.T) {
	t.Parallel()
	basePort := 29080
	maxAttempts := 3

	// Occupy all ports in the range we'll test
	listeners := make([]net.Listener, maxAttempts)
	for i := 0; i < maxAttempts; i++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", basePort+i))
		if err != nil {
			t.Skipf("could not occupy port %d for test: %v", basePort+i, err)
		}
		listeners[i] = ln
		defer func(l net.Listener) { _ = l.Close() }(ln)
	}

	// Now try to find a port with max attempts in that occupied range
	_, _, err := findAvailablePort("", basePort, maxAttempts)
	if err == nil {
		t.Error("expected error when all ports are busy")
	}
}
