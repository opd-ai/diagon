package profile

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestBuildInjectedConfigBundleSuccess(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	paywallServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(paywallServer.Close)

	paywallHost, paywallPort, err := validateServiceURL(paywallServer.URL)
	if err != nil {
		t.Fatalf("parse paywall url: %v", err)
	}
	paywallListen := net.JoinHostPort(paywallHost, strconv.Itoa(paywallPort))

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{Name: "i2pd", Listen: "127.0.0.1:7070", HealthURL: "http://127.0.0.1:7070/health", StartupOrder: 1},
			{Name: "paywall", Listen: paywallListen, HealthURL: paywallServer.URL, DependsOn: []string{"i2pd"}, StartupOrder: 2},
			{Name: "store", Listen: "127.0.0.1:8080", HealthURL: "http://127.0.0.1:8080/healthz", DependsOn: []string{"i2pd", "paywall"}, StartupOrder: 3},
		},
		APILinks: []APILink{{From: "store", To: "paywall", Endpoint: paywallServer.URL + "/api/v1/payments"}},
		I2PDTunnels: []I2PDTunnel{
			{Name: "store-http", Type: "http", Listen: "127.0.0.1:18080", Target: "127.0.0.1:8080", TargetService: "store"},
			{Name: "paywall-http", Type: "http", Listen: "127.0.0.1:18081", Target: paywallListen, TargetService: "paywall"},
		},
	}

	bootstrap := BootstrapProfile{
		Name:                "local-single-host-bootstrap",
		ServiceContractFile: "service-contract.json",
		StartupSequence:     []string{"i2pd", "paywall", "store", "diagonctl"},
		ExpectedTunnels:     []string{"store-http", "paywall-http"},
		Components: []BootstrapComponent{
			{Name: "i2pd", Listen: "127.0.0.1:7070", HealthURL: "http://127.0.0.1:7070/health", ConfigPath: "/etc/diagon/i2pd/i2pd.conf", Settings: map[string]string{"tunnel_config_path": "/etc/diagon/i2pd/tunnels.conf"}},
			{Name: "paywall", Listen: paywallListen, HealthURL: paywallServer.URL, ConfigPath: "/etc/diagon/paywall/config.yaml", WalletMode: "stubbed", SecretRefs: []string{"PAYWALL_TOKEN"}, Settings: map[string]string{"wallet_rpc_url": "http://127.0.0.1:18089/json_rpc"}},
			{Name: "store", Listen: "127.0.0.1:8080", HealthURL: "http://127.0.0.1:8080/healthz", ConfigPath: "/etc/diagon/store/config.yaml", SecretRefs: []string{"STORE_SESSION_SECRET"}, Settings: map[string]string{"paywall_endpoint": paywallServer.URL + "/api/v1/payments"}},
		},
		Secrets: []BootstrapSecret{{Name: "PAYWALL_TOKEN", Source: "env", Ref: "PAYWALL_TOKEN", Required: true}, {Name: "STORE_SESSION_SECRET", Source: "file", Ref: filepath.Join(tempDir, "store-session") + "-secret", Required: true}},
	}

	bundle, err := BuildInjectedConfigBundle(bootstrap, contract)
	if err != nil {
		t.Fatalf("BuildInjectedConfigBundle() returned error: %v", err)
	}

	if bundle.Store.PaywallEndpoint != paywallServer.URL+"/api/v1/payments" {
		t.Fatalf("unexpected store paywall endpoint: %q", bundle.Store.PaywallEndpoint)
	}
	if bundle.Paywall.ListenPort != paywallPort {
		t.Fatalf("expected paywall listen port %d, got %d", paywallPort, bundle.Paywall.ListenPort)
	}
	if len(bundle.I2PD.Tunnels) != 2 {
		t.Fatalf("expected 2 injected tunnels, got %d", len(bundle.I2PD.Tunnels))
	}
}

func TestPhase2IntegrationStoreCallsPaywallViaConfiguredLocalEndpoint(t *testing.T) {
	t.Parallel()

	var paywallCalls atomic.Int64
	paywallServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/payments" {
			http.NotFound(w, r)
			return
		}
		paywallCalls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(paywallServer.Close)

	paywallHost, paywallPort, err := validateServiceURL(paywallServer.URL)
	if err != nil {
		t.Fatalf("parse paywall url: %v", err)
	}
	paywallListen := net.JoinHostPort(paywallHost, strconv.Itoa(paywallPort))

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{Name: "i2pd", Listen: "127.0.0.1:7070", HealthURL: "http://127.0.0.1:7070/health", StartupOrder: 1},
			{Name: "paywall", Listen: paywallListen, HealthURL: paywallServer.URL, DependsOn: []string{"i2pd"}, StartupOrder: 2},
			{Name: "store", Listen: "127.0.0.1:8080", HealthURL: "http://127.0.0.1:8080/healthz", DependsOn: []string{"i2pd", "paywall"}, StartupOrder: 3},
		},
		APILinks: []APILink{{From: "store", To: "paywall", Endpoint: paywallServer.URL + "/api/v1/payments"}},
		I2PDTunnels: []I2PDTunnel{
			{Name: "store-http", Type: "http", Listen: "127.0.0.1:18080", Target: "127.0.0.1:8080", TargetService: "store"},
			{Name: "paywall-http", Type: "http", Listen: "127.0.0.1:18081", Target: paywallListen, TargetService: "paywall"},
		},
	}

	bootstrap := BootstrapProfile{
		Name:                "phase2-test",
		ServiceContractFile: "service-contract.json",
		StartupSequence:     []string{"i2pd", "paywall", "store", "diagonctl"},
		ExpectedTunnels:     []string{"store-http", "paywall-http"},
		Components: []BootstrapComponent{
			{Name: "i2pd", Listen: "127.0.0.1:7070", HealthURL: "http://127.0.0.1:7070/health", ConfigPath: "/etc/diagon/i2pd/i2pd.conf", Settings: map[string]string{"tunnel_config_path": "/etc/diagon/i2pd/tunnels.conf"}},
			{Name: "paywall", Listen: paywallListen, HealthURL: paywallServer.URL, ConfigPath: "/etc/diagon/paywall/config.yaml", WalletMode: "stubbed", Settings: map[string]string{"wallet_rpc_url": "http://127.0.0.1:18089/json_rpc"}},
			{Name: "store", Listen: "127.0.0.1:8080", HealthURL: "http://127.0.0.1:8080/healthz", ConfigPath: "/etc/diagon/store/config.yaml", Settings: map[string]string{"paywall_endpoint": paywallServer.URL + "/api/v1/payments"}},
		},
	}

	bundle, err := BuildInjectedConfigBundle(bootstrap, contract)
	if err != nil {
		t.Fatalf("BuildInjectedConfigBundle() returned error: %v", err)
	}

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkout" {
			http.NotFound(w, r)
			return
		}
		resp, err := http.Post(bundle.Store.PaywallEndpoint, "application/json", bytes.NewBufferString(`{"amount":1}`))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusOK {
			http.Error(w, "paywall not ready", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(storeServer.Close)

	resp, err := http.Get(storeServer.URL + "/checkout")
	if err != nil {
		t.Fatalf("call store checkout endpoint: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected store checkout to succeed, got status %d", resp.StatusCode)
	}
	if paywallCalls.Load() != 1 {
		t.Fatalf("expected store to call paywall exactly once, got %d", paywallCalls.Load())
	}
}

func TestPhase2IntegrationTrafficPathWorksWithI2PDEnabledRouting(t *testing.T) {
	t.Parallel()

	storeAddr := reserveLoopbackAddr(t)
	paywallAddr := reserveLoopbackAddr(t)
	storeTunnelAddr := reserveLoopbackAddr(t)
	paywallTunnelAddr := reserveLoopbackAddr(t)

	paywallURL := "http://" + paywallAddr
	storeURL := "http://" + storeAddr

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{Name: "i2pd", Listen: "127.0.0.1:7070", HealthURL: "http://127.0.0.1:7070/health", StartupOrder: 1},
			{Name: "paywall", Listen: paywallAddr, HealthURL: paywallURL + "/healthz", DependsOn: []string{"i2pd"}, StartupOrder: 2},
			{Name: "store", Listen: storeAddr, HealthURL: storeURL + "/healthz", DependsOn: []string{"i2pd", "paywall"}, StartupOrder: 3},
		},
		APILinks: []APILink{{From: "store", To: "paywall", Endpoint: paywallURL + "/api/v1/payments"}},
		I2PDTunnels: []I2PDTunnel{
			{Name: "store-http", Type: "http", Listen: storeTunnelAddr, Target: storeAddr, TargetService: "store"},
			{Name: "paywall-http", Type: "http", Listen: paywallTunnelAddr, Target: paywallAddr, TargetService: "paywall"},
		},
	}

	bootstrap := BootstrapProfile{
		Name:                "phase2-routing-test",
		ServiceContractFile: "service-contract.json",
		StartupSequence:     []string{"i2pd", "paywall", "store", "diagonctl"},
		ExpectedTunnels:     []string{"store-http", "paywall-http"},
		Components: []BootstrapComponent{
			{Name: "i2pd", Listen: "127.0.0.1:7070", HealthURL: "http://127.0.0.1:7070/health", ConfigPath: "/etc/diagon/i2pd/i2pd.conf", Settings: map[string]string{"tunnel_config_path": "/etc/diagon/i2pd/tunnels.conf"}},
			{Name: "paywall", Listen: paywallAddr, HealthURL: paywallURL + "/healthz", ConfigPath: "/etc/diagon/paywall/config.yaml", WalletMode: "stubbed", Settings: map[string]string{"wallet_rpc_url": "http://127.0.0.1:18089/json_rpc"}},
			{Name: "store", Listen: storeAddr, HealthURL: storeURL + "/healthz", ConfigPath: "/etc/diagon/store/config.yaml", Settings: map[string]string{"paywall_endpoint": paywallURL + "/api/v1/payments"}},
		},
	}

	bundle, err := BuildInjectedConfigBundle(bootstrap, contract)
	if err != nil {
		t.Fatalf("BuildInjectedConfigBundle() returned error: %v", err)
	}
	if len(bundle.Store.I2PPaywallRouteEndpoints) == 0 {
		t.Fatal("expected generated i2pd paywall route endpoint")
	}

	var paywallCalls atomic.Int64
	paywallStop := startHTTPServerOnAddr(t, paywallAddr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/api/v1/payments":
			if r.URL.Path == "/api/v1/payments" {
				paywallCalls.Add(1)
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer paywallStop()

	storeStop := startHTTPServerOnAddr(t, storeAddr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
		case "/checkout":
			paywallEndpoint := bundle.Store.I2PPaywallRouteEndpoints[0] + "/api/v1/payments"
			resp, err := http.Post(paywallEndpoint, "application/json", bytes.NewBufferString(`{"amount":2}`))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)
			if resp.StatusCode != http.StatusOK {
				http.Error(w, "paywall route failed", http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer storeStop()

	storeTunnelStop := startReverseProxyOnAddr(t, storeTunnelAddr, storeURL)
	defer storeTunnelStop()
	paywallTunnelStop := startReverseProxyOnAddr(t, paywallTunnelAddr, paywallURL)
	defer paywallTunnelStop()

	resp, err := http.Get("http://" + storeTunnelAddr + "/checkout")
	if err != nil {
		t.Fatalf("call store through i2pd tunnel: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected i2pd-routed checkout to succeed, got %d", resp.StatusCode)
	}
	if paywallCalls.Load() != 1 {
		t.Fatalf("expected one paywall call through i2pd route, got %d", paywallCalls.Load())
	}
}

func TestBuildInjectedConfigBundleRejectsStorePaywallMismatch(t *testing.T) {
	t.Parallel()

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{Name: "i2pd", Listen: "127.0.0.1:7070", HealthURL: "http://127.0.0.1:7070/health", StartupOrder: 1},
			{Name: "paywall", Listen: "127.0.0.1:8081", HealthURL: "http://127.0.0.1:8081/healthz", DependsOn: []string{"i2pd"}, StartupOrder: 2},
			{Name: "store", Listen: "127.0.0.1:8080", HealthURL: "http://127.0.0.1:8080/healthz", DependsOn: []string{"i2pd", "paywall"}, StartupOrder: 3},
		},
		APILinks: []APILink{{From: "store", To: "paywall", Endpoint: "http://127.0.0.1:8081/api/v1/payments"}},
		I2PDTunnels: []I2PDTunnel{
			{Name: "store-http", Type: "http", Listen: "127.0.0.1:18080", Target: "127.0.0.1:8080", TargetService: "store"},
			{Name: "paywall-http", Type: "http", Listen: "127.0.0.1:18081", Target: "127.0.0.1:8081", TargetService: "paywall"},
		},
	}

	bootstrap := BootstrapProfile{
		Name:                "phase2-invalid",
		ServiceContractFile: "service-contract.json",
		StartupSequence:     []string{"i2pd", "paywall", "store", "diagonctl"},
		ExpectedTunnels:     []string{"store-http", "paywall-http"},
		Components: []BootstrapComponent{
			{Name: "i2pd", Listen: "127.0.0.1:7070", HealthURL: "http://127.0.0.1:7070/health", ConfigPath: "/etc/diagon/i2pd/i2pd.conf", Settings: map[string]string{"tunnel_config_path": "/etc/diagon/i2pd/tunnels.conf"}},
			{Name: "paywall", Listen: "127.0.0.1:8081", HealthURL: "http://127.0.0.1:8081/healthz", ConfigPath: "/etc/diagon/paywall/config.yaml", WalletMode: "stubbed", Settings: map[string]string{"wallet_rpc_url": "http://127.0.0.1:18089/json_rpc"}},
			{Name: "store", Listen: "127.0.0.1:8080", HealthURL: "http://127.0.0.1:8080/healthz", ConfigPath: "/etc/diagon/store/config.yaml", Settings: map[string]string{"paywall_endpoint": "http://127.0.0.1:9999/api/v1/payments"}},
		},
	}

	if _, err := BuildInjectedConfigBundle(bootstrap, contract); err == nil {
		t.Fatal("expected paywall endpoint mismatch error, got nil")
	}
}

func reserveLoopbackAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve loopback addr: %v", err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func startHTTPServerOnAddr(t *testing.T, addr string, handler http.Handler) func() {
	t.Helper()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen on %s: %v", addr, err)
	}

	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(ln)
	}()

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}
}

func startReverseProxyOnAddr(t *testing.T, listenAddr, targetURL string) func() {
	t.Helper()

	target, err := url.Parse(targetURL)
	if err != nil {
		t.Fatalf("parse target url %q: %v", targetURL, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	return startHTTPServerOnAddr(t, listenAddr, proxy)
}
