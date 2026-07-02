// Command diagon-stub-service runs a minimal stub HTTP service used during CI
// integration bootstrap to stand in for i2pd, Store, Paywall, or their tunnels.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/opd-ai/diagon/internal/citools"
)

func main() {
	var (
		host       string
		port       int
		readyDelay float64
	)

	flag.StringVar(&host, "host", "127.0.0.1", "host/interface to bind the stub service to")
	flag.IntVar(&port, "port", 0, "port to bind the stub service to (required)")
	flag.Float64Var(&readyDelay, "ready-delay", 0, "seconds before the stub reports ready (returns 503 until elapsed)")
	flag.Parse()

	if port <= 0 {
		fmt.Fprintln(os.Stderr, "stub service error: --port is required and must be positive")
		os.Exit(2)
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	delay := time.Duration(readyDelay * float64(time.Second))

	if err := citools.RunStubService(citools.StubServiceOptions{Addr: addr, ReadyDelay: delay}); err != nil {
		fmt.Fprintf(os.Stderr, "stub service error: %v\n", err)
		os.Exit(1)
	}
}
