package citools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opd-ai/diagon/internal/profile"
)

const smokeHTTPTimeout = 5 * time.Second

// SmokeHarnessOptions configures the end-to-end transaction smoke harness.
type SmokeHarnessOptions struct {
	// PlanPath is the path to the emitted release-candidate smoke plan JSON.
	PlanPath string
	// OutputPath is where the smoke result JSON is written.
	OutputPath string
	// ExpectedWalletMode, when non-empty, is asserted against the plan wallet mode.
	ExpectedWalletMode string
}

type smokeResultEntry struct {
	StatusCode int             `json:"status_code"`
	Response   json.RawMessage `json:"response"`
	WalletMode string          `json:"wallet_mode,omitempty"`
}

type smokeReport struct {
	Initial          smokeResultEntry `json:"initial"`
	AfterRestart     smokeResultEntry `json:"after_restart"`
	RestartValidated bool             `json:"restart_validated"`
}

type checkoutResponse struct {
	Checkout string `json:"checkout"`
	Paywall  struct {
		Settled bool `json:"settled"`
	} `json:"paywall"`
}

// derivedEndpoints holds the concrete addresses and request paths extracted from
// the smoke plan.
type derivedEndpoints struct {
	i2pdAddr    string
	paywallAddr string
	storeAddr   string

	storeTunnelAddr   string
	paywallTunnelAddr string

	i2pdHealthPath    string
	paywallHealthPath string
	storeHealthPath   string

	paywallSmokePath   string
	storeCheckoutPath  string
	paywallValidateURL string
}

// RunSmokeHarness loads the emitted smoke plan, stands up stub origin services and
// i2pd tunnel proxies, executes the marketplace transaction path (including a
// graceful restart cycle), writes the smoke result, and validates the success
// criteria. It returns an error if any check fails.
func RunSmokeHarness(opts SmokeHarnessOptions) error {
	if strings.TrimSpace(opts.PlanPath) == "" {
		return fmt.Errorf("smoke plan path cannot be empty")
	}
	if strings.TrimSpace(opts.OutputPath) == "" {
		return fmt.Errorf("smoke output path cannot be empty")
	}

	plan, err := loadSmokePlan(opts.PlanPath)
	if err != nil {
		return err
	}

	if expected := strings.TrimSpace(opts.ExpectedWalletMode); expected != "" && plan.WalletMode != expected {
		return fmt.Errorf("smoke plan wallet_mode %q does not match expected %q", plan.WalletMode, expected)
	}

	endpoints, err := deriveEndpoints(plan)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: smokeHTTPTimeout}

	// First cycle: boot, health, marketplace access.
	servers, err := startServers(endpoints)
	if err != nil {
		return err
	}
	if err := runHealthChecks(client, plan.HealthChecks); err != nil {
		servers.Close()
		return err
	}
	initial, err := marketplaceAccess(client, plan, plan.WalletMode)
	if err != nil {
		servers.Close()
		return err
	}
	servers.Close()

	// Second cycle: graceful restart, health, marketplace access again.
	servers, err = startServers(endpoints)
	if err != nil {
		return err
	}
	if err := runHealthChecks(client, plan.GracefulRestart.PostRestartChecks); err != nil {
		servers.Close()
		return err
	}
	afterRestart, err := marketplaceAccess(client, plan, "")
	if err != nil {
		servers.Close()
		return err
	}
	servers.Close()

	report := smokeReport{
		Initial:          initial,
		AfterRestart:     afterRestart,
		RestartValidated: true,
	}

	if err := writeSmokeReport(opts.OutputPath, report); err != nil {
		return err
	}

	return validateSmokeReport(report)
}

func loadSmokePlan(path string) (profile.ReleaseCandidateSmokePlan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return profile.ReleaseCandidateSmokePlan{}, fmt.Errorf("read smoke plan %s: %w", path, err)
	}
	var plan profile.ReleaseCandidateSmokePlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return profile.ReleaseCandidateSmokePlan{}, fmt.Errorf("parse smoke plan %s: %w", path, err)
	}
	return plan, nil
}

func deriveEndpoints(plan profile.ReleaseCandidateSmokePlan) (derivedEndpoints, error) {
	services := make(map[string]profile.ReleaseCandidateServiceEndpoint, len(plan.ServiceEndpoints))
	for _, endpoint := range plan.ServiceEndpoints {
		services[strings.TrimSpace(endpoint.Service)] = endpoint
	}
	tunnels := make(map[string]profile.ReleaseCandidateTunnelEndpoint, len(plan.TunnelEndpoints))
	for _, tunnel := range plan.TunnelEndpoints {
		tunnels[strings.TrimSpace(tunnel.TargetService)] = tunnel
	}

	required := []string{"i2pd", "paywall", "store"}
	for _, name := range required {
		if _, ok := services[name]; !ok {
			return derivedEndpoints{}, fmt.Errorf("smoke plan missing service endpoint %q", name)
		}
	}
	for _, name := range []string{"store", "paywall"} {
		if _, ok := tunnels[name]; !ok {
			return derivedEndpoints{}, fmt.Errorf("smoke plan missing tunnel endpoint for %q", name)
		}
	}

	i2pdHealth, err := urlPath(services["i2pd"].HealthURL)
	if err != nil {
		return derivedEndpoints{}, err
	}
	paywallHealth, err := urlPath(services["paywall"].HealthURL)
	if err != nil {
		return derivedEndpoints{}, err
	}
	storeHealth, err := urlPath(services["store"].HealthURL)
	if err != nil {
		return derivedEndpoints{}, err
	}
	paywallSmoke, err := urlPath(plan.PaywallValidation.URL)
	if err != nil {
		return derivedEndpoints{}, err
	}
	storeCheckout, err := urlPath(plan.MarketplaceAccess.URL)
	if err != nil {
		return derivedEndpoints{}, err
	}

	return derivedEndpoints{
		i2pdAddr:           strings.TrimSpace(services["i2pd"].Listen),
		paywallAddr:        strings.TrimSpace(services["paywall"].Listen),
		storeAddr:          strings.TrimSpace(services["store"].Listen),
		storeTunnelAddr:    strings.TrimSpace(tunnels["store"].Listen),
		paywallTunnelAddr:  strings.TrimSpace(tunnels["paywall"].Listen),
		i2pdHealthPath:     i2pdHealth,
		paywallHealthPath:  paywallHealth,
		storeHealthPath:    storeHealth,
		paywallSmokePath:   paywallSmoke,
		storeCheckoutPath:  storeCheckout,
		paywallValidateURL: strings.TrimSpace(plan.PaywallValidation.URL),
	}, nil
}

func urlPath(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse url %q: %w", rawURL, err)
	}
	if parsed.Path == "" {
		return "/", nil
	}
	return parsed.Path, nil
}

type serverSet struct {
	servers []*http.Server
}

func (s *serverSet) Close() {
	for _, server := range s.servers {
		_ = server.Close()
	}
}

func startServers(endpoints derivedEndpoints) (*serverSet, error) {
	set := &serverSet{}

	// Origin services.
	if err := set.listenAndServe(endpoints.i2pdAddr, newI2PDHandler(endpoints)); err != nil {
		set.Close()
		return nil, err
	}
	if err := set.listenAndServe(endpoints.paywallAddr, newPaywallHandler(endpoints)); err != nil {
		set.Close()
		return nil, err
	}
	if err := set.listenAndServe(endpoints.storeAddr, newStoreHandler(endpoints)); err != nil {
		set.Close()
		return nil, err
	}

	// i2pd tunnel proxies forward to their origin services.
	if err := set.listenAndServe(endpoints.storeTunnelAddr, newProxyHandler(endpoints.storeAddr)); err != nil {
		set.Close()
		return nil, err
	}
	if err := set.listenAndServe(endpoints.paywallTunnelAddr, newProxyHandler(endpoints.paywallAddr)); err != nil {
		set.Close()
		return nil, err
	}

	return set, nil
}

func (s *serverSet) listenAndServe(addr string, handler http.Handler) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	s.servers = append(s.servers, server)
	go func() {
		_ = server.Serve(listener)
	}()
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func newI2PDHandler(endpoints derivedEndpoints) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == endpoints.i2pdHealthPath {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

func newPaywallHandler(endpoints derivedEndpoints) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == endpoints.paywallHealthPath:
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == endpoints.paywallSmokePath:
			writeJSON(w, http.StatusOK, map[string]bool{"settled": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func newStoreHandler(endpoints derivedEndpoints) http.Handler {
	client := &http.Client{Timeout: smokeHTTPTimeout}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == endpoints.storeHealthPath:
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == endpoints.storeCheckoutPath:
			paywallPayload, err := postJSON(client, endpoints.paywallValidateURL, []byte("{}"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"checkout": "ok",
				"paywall":  json.RawMessage(paywallPayload),
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func newProxyHandler(targetAddr string) http.Handler {
	client := &http.Client{Timeout: smokeHTTPTimeout}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetURL := fmt.Sprintf("http://%s%s", targetAddr, r.URL.RequestURI())

		var body []byte
		if r.Body != nil {
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			body = data
		}

		req, err := http.NewRequest(r.Method, targetURL, bytes.NewReader(body))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if ct := r.Header.Get("Content-Type"); ct != "" {
			req.Header.Set("Content-Type", ct)
		}

		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		payload, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(payload)
	})
}

func runHealthChecks(client *http.Client, checks []profile.ReleaseCandidateSmokeCheck) error {
	for _, check := range checks {
		method := check.Method
		if method == "" {
			method = http.MethodGet
		}
		req, err := http.NewRequest(method, check.URL, nil)
		if err != nil {
			return fmt.Errorf("build health request %q: %w", check.URL, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("health check %q failed: %w", check.URL, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != check.ExpectedStatus {
			return fmt.Errorf("health check %q returned %d, expected %d", check.URL, resp.StatusCode, check.ExpectedStatus)
		}
	}
	return nil
}

func marketplaceAccess(client *http.Client, plan profile.ReleaseCandidateSmokePlan, walletMode string) (smokeResultEntry, error) {
	access := plan.MarketplaceAccess
	method := access.Method
	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequest(method, access.URL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return smokeResultEntry{}, fmt.Errorf("build marketplace request %q: %w", access.URL, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return smokeResultEntry{}, fmt.Errorf("marketplace access %q failed: %w", access.URL, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return smokeResultEntry{}, fmt.Errorf("read marketplace response: %w", err)
	}

	return smokeResultEntry{
		StatusCode: resp.StatusCode,
		Response:   json.RawMessage(payload),
		WalletMode: walletMode,
	}, nil
}

func postJSON(client *http.Client, rawURL string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request %q: %w", rawURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %q failed: %w", rawURL, err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func writeSmokeReport(path string, report smokeReport) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory %s: %w", dir, err)
		}
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode smoke report: %w", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write smoke report %s: %w", path, err)
	}
	return nil
}

func validateSmokeReport(report smokeReport) error {
	if report.Initial.StatusCode != http.StatusOK {
		return fmt.Errorf("initial marketplace access returned %d, expected 200", report.Initial.StatusCode)
	}
	if report.AfterRestart.StatusCode != http.StatusOK {
		return fmt.Errorf("post-restart marketplace access returned %d, expected 200", report.AfterRestart.StatusCode)
	}

	for label, entry := range map[string]smokeResultEntry{"initial": report.Initial, "after_restart": report.AfterRestart} {
		var parsed checkoutResponse
		if err := json.Unmarshal(entry.Response, &parsed); err != nil {
			return fmt.Errorf("parse %s checkout response: %w", label, err)
		}
		if !parsed.Paywall.Settled {
			return fmt.Errorf("%s checkout paywall settlement was not true", label)
		}
	}

	return nil
}
