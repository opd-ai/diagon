package profile

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProbeServiceContractDefinitionSuccess(t *testing.T) {
	t.Parallel()

	i2pdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(i2pdServer.Close)

	paywallServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(paywallServer.Close)

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(storeServer.Close)

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{
				Name:         "i2pd",
				Listen:       mustAddr(t, i2pdServer.URL),
				HealthURL:    i2pdServer.URL,
				StartupOrder: 1,
			},
			{
				Name:         "paywall",
				Listen:       mustAddr(t, paywallServer.URL),
				HealthURL:    paywallServer.URL,
				DependsOn:    []string{"i2pd"},
				StartupOrder: 2,
			},
			{
				Name:         "store",
				Listen:       mustAddr(t, storeServer.URL),
				HealthURL:    storeServer.URL,
				DependsOn:    []string{"i2pd", "paywall"},
				StartupOrder: 3,
			},
		},
		APILinks: []APILink{{
			From:     "store",
			To:       "paywall",
			Endpoint: paywallServer.URL + "/api/v1/payments",
		}},
		I2PDTunnels: []I2PDTunnel{
			{
				Name:          "store-http",
				Type:          "http",
				Listen:        "127.0.0.1:18080",
				Target:        mustAddr(t, storeServer.URL),
				TargetService: "store",
			},
			{
				Name:          "paywall-http",
				Type:          "http",
				Listen:        "127.0.0.1:18081",
				Target:        mustAddr(t, paywallServer.URL),
				TargetService: "paywall",
			},
		},
	}

	result := ProbeServiceContractDefinition(contract, RuntimeProbeOptions{
		Timeout:  2 * time.Second,
		Interval: 20 * time.Millisecond,
	})
	if result.HasErrors() {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
}

func TestProbeServiceContractDefinitionTimeout(t *testing.T) {
	t.Parallel()

	i2pdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(i2pdServer.Close)

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(storeServer.Close)

	missingPort := freeTCPPort(t)

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{
				Name:         "i2pd",
				Listen:       mustAddr(t, i2pdServer.URL),
				HealthURL:    i2pdServer.URL,
				StartupOrder: 1,
			},
			{
				Name:         "paywall",
				Listen:       "127.0.0.1:" + missingPort,
				HealthURL:    "http://127.0.0.1:" + missingPort + "/healthz",
				DependsOn:    []string{"i2pd"},
				StartupOrder: 2,
			},
			{
				Name:         "store",
				Listen:       mustAddr(t, storeServer.URL),
				HealthURL:    storeServer.URL,
				DependsOn:    []string{"i2pd", "paywall"},
				StartupOrder: 3,
			},
		},
		APILinks: []APILink{{
			From:     "store",
			To:       "paywall",
			Endpoint: "http://127.0.0.1:" + missingPort + "/api/v1/payments",
		}},
		I2PDTunnels: []I2PDTunnel{
			{
				Name:          "store-http",
				Type:          "http",
				Listen:        "127.0.0.1:18080",
				Target:        mustAddr(t, storeServer.URL),
				TargetService: "store",
			},
			{
				Name:          "paywall-http",
				Type:          "http",
				Listen:        "127.0.0.1:18081",
				Target:        "127.0.0.1:" + missingPort,
				TargetService: "paywall",
			},
		},
	}

	result := ProbeServiceContractDefinition(contract, RuntimeProbeOptions{
		Timeout:  250 * time.Millisecond,
		Interval: 25 * time.Millisecond,
	})
	if !result.HasErrors() {
		t.Fatal("expected probe errors, got none")
	}

	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "paywall") || !strings.Contains(joined, "failed runtime readiness probe") {
		t.Fatalf("expected paywall runtime probe timeout error, got: %s", joined)
	}
}

func TestProbeServiceContractDefinitionDependencySequenceViolation(t *testing.T) {
	t.Parallel()

	i2pdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(i2pdServer.Close)

	paywallServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(220 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(paywallServer.Close)

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(storeServer.Close)

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{
				Name:         "i2pd",
				Listen:       mustAddr(t, i2pdServer.URL),
				HealthURL:    i2pdServer.URL,
				StartupOrder: 1,
			},
			{
				Name:         "paywall",
				Listen:       mustAddr(t, paywallServer.URL),
				HealthURL:    paywallServer.URL,
				DependsOn:    []string{"i2pd"},
				StartupOrder: 2,
			},
			{
				Name:         "store",
				Listen:       mustAddr(t, storeServer.URL),
				HealthURL:    storeServer.URL,
				DependsOn:    []string{"i2pd", "paywall"},
				StartupOrder: 3,
			},
		},
		APILinks: []APILink{{
			From:     "store",
			To:       "paywall",
			Endpoint: paywallServer.URL + "/api/v1/payments",
		}},
		I2PDTunnels: []I2PDTunnel{
			{
				Name:          "store-http",
				Type:          "http",
				Listen:        "127.0.0.1:18080",
				Target:        mustAddr(t, storeServer.URL),
				TargetService: "store",
			},
			{
				Name:          "paywall-http",
				Type:          "http",
				Listen:        "127.0.0.1:18081",
				Target:        mustAddr(t, paywallServer.URL),
				TargetService: "paywall",
			},
		},
	}

	result := ProbeServiceContractDefinition(contract, RuntimeProbeOptions{
		Timeout:  2 * time.Second,
		Interval: 20 * time.Millisecond,
	})
	if !result.HasErrors() {
		t.Fatal("expected dependency sequencing errors, got none")
	}

	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "reported readiness before dependency \"paywall\"") {
		t.Fatalf("expected dependency sequencing violation, got: %s", joined)
	}
}

func mustAddr(t *testing.T, rawURL string) string {
	t.Helper()

	host, port, err := validateServiceURL(rawURL)
	if err != nil {
		t.Fatalf("parse url %q: %v", rawURL, err)
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func freeTCPPort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate free tcp port: %v", err)
	}
	defer listener.Close()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split tcp port: %v", err)
	}

	return port
}
