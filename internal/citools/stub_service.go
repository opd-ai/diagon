// Package citools contains CI integration helpers that were previously embedded
// as inline Python programs inside the GitHub Actions workflow. Keeping this
// logic in Go makes it buildable, vettable, and unit-testable alongside the rest
// of the module.
package citools

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// StubServiceOptions configures a single stub HTTP service used to stand in for
// i2pd, Store, Paywall, or their tunnels during CI integration bootstrap.
type StubServiceOptions struct {
	// Addr is the host:port the stub listens on (for example 127.0.0.1:7070).
	Addr string
	// ReadyDelay delays readiness: until it elapses the stub returns HTTP 503 so
	// that readiness/backoff probes can be exercised.
	ReadyDelay time.Duration
}

type stubHandler struct {
	readyAfter time.Time
}

func newStubHandler(readyDelay time.Duration) *stubHandler {
	return &stubHandler{readyAfter: time.Now().Add(readyDelay)}
}

func (h *stubHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if time.Now().Before(h.readyAfter) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"starting"}`))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// RunStubService starts a stub HTTP service and blocks serving requests until the
// process is terminated. It returns an error only if the listener cannot be
// created or the server stops unexpectedly.
func RunStubService(opts StubServiceOptions) error {
	if strings.TrimSpace(opts.Addr) == "" {
		return fmt.Errorf("stub service address cannot be empty")
	}
	if opts.ReadyDelay < 0 {
		opts.ReadyDelay = 0
	}

	listener, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", opts.Addr, err)
	}

	server := &http.Server{
		Handler:           newStubHandler(opts.ReadyDelay),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server.Serve(listener)
}
