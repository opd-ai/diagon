package citools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opd-ai/diagon/internal/profile"
)

func TestStubHandlerReadyReturnsOK(t *testing.T) {
	server := httptest.NewServer(newStubHandler(0))
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"status":"ok"}` {
		t.Fatalf("body = %q, want ready payload", string(body))
	}
}

func TestStubHandlerNotReadyReturns503(t *testing.T) {
	server := httptest.NewServer(newStubHandler(10 * time.Second))
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

func TestRunStubServiceRejectsEmptyAddr(t *testing.T) {
	if err := RunStubService(StubServiceOptions{}); err == nil {
		t.Fatal("expected error for empty address")
	}
}

func TestRunStubServiceServesRequests(t *testing.T) {
	const addr = "127.0.0.1:39231"
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunStubService(StubServiceOptions{Addr: addr})
	}()

	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			lastErr = nil
		} else {
			lastErr = err
		}
		select {
		case err := <-errCh:
			t.Fatalf("stub service exited early: %v", err)
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("stub service never became ready: %v", lastErr)
}

func TestRunSmokeHarnessSucceeds(t *testing.T) {
	plan := buildTestSmokePlan()
	planPath := filepath.Join(t.TempDir(), "smoke-plan.json")
	writePlan(t, planPath, plan)

	outPath := filepath.Join(t.TempDir(), "smoke-result.json")
	if err := RunSmokeHarness(SmokeHarnessOptions{
		PlanPath:           planPath,
		OutputPath:         outPath,
		ExpectedWalletMode: "stubbed",
	}); err != nil {
		t.Fatalf("RunSmokeHarness: %v", err)
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var report smokeReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if !report.RestartValidated {
		t.Fatal("restart_validated should be true")
	}
	if report.Initial.StatusCode != 200 || report.AfterRestart.StatusCode != 200 {
		t.Fatalf("status codes = %d/%d, want 200/200", report.Initial.StatusCode, report.AfterRestart.StatusCode)
	}
	if report.Initial.WalletMode != "stubbed" {
		t.Fatalf("initial wallet_mode = %q, want stubbed", report.Initial.WalletMode)
	}
}

func TestRunSmokeHarnessWalletModeMismatch(t *testing.T) {
	plan := buildTestSmokePlan()
	planPath := filepath.Join(t.TempDir(), "smoke-plan.json")
	writePlan(t, planPath, plan)

	err := RunSmokeHarness(SmokeHarnessOptions{
		PlanPath:           planPath,
		OutputPath:         filepath.Join(t.TempDir(), "out.json"),
		ExpectedWalletMode: "production",
	})
	if err == nil {
		t.Fatal("expected wallet mode mismatch error")
	}
}

func TestRunSmokeHarnessMissingPlan(t *testing.T) {
	err := RunSmokeHarness(SmokeHarnessOptions{
		PlanPath:   filepath.Join(t.TempDir(), "does-not-exist.json"),
		OutputPath: filepath.Join(t.TempDir(), "out.json"),
	})
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
}

func writePlan(t *testing.T, path string, plan any) {
	t.Helper()
	body, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
}

func buildTestSmokePlan() profile.ReleaseCandidateSmokePlan {
	const (
		i2pdListen          = "127.0.0.1:39301"
		paywallListen       = "127.0.0.1:39302"
		storeListen         = "127.0.0.1:39303"
		storeTunnelListen   = "127.0.0.1:39304"
		paywallTunnelListen = "127.0.0.1:39305"
	)

	healthChecks := []profile.ReleaseCandidateSmokeCheck{
		{Service: "i2pd", Method: "GET", URL: "http://" + i2pdListen + "/health", ExpectedStatus: 200},
		{Service: "paywall", Method: "GET", URL: "http://" + paywallListen + "/healthz", ExpectedStatus: 200},
		{Service: "store", Method: "GET", URL: "http://" + storeListen + "/healthz", ExpectedStatus: 200},
	}

	return profile.ReleaseCandidateSmokePlan{
		Name:       "test-smoke",
		WalletMode: "stubbed",
		ServiceEndpoints: []profile.ReleaseCandidateServiceEndpoint{
			{Service: "i2pd", Listen: i2pdListen, BaseURL: "http://" + i2pdListen, HealthURL: "http://" + i2pdListen + "/health"},
			{Service: "paywall", Listen: paywallListen, BaseURL: "http://" + paywallListen, HealthURL: "http://" + paywallListen + "/healthz"},
			{Service: "store", Listen: storeListen, BaseURL: "http://" + storeListen, HealthURL: "http://" + storeListen + "/healthz"},
		},
		TunnelEndpoints: []profile.ReleaseCandidateTunnelEndpoint{
			{Name: "store-http", TargetService: "store", Listen: storeTunnelListen, Target: storeListen, Type: "http"},
			{Name: "paywall-http", TargetService: "paywall", Listen: paywallTunnelListen, Target: paywallListen, Type: "http"},
		},
		HealthChecks: healthChecks,
		MarketplaceAccess: profile.ReleaseCandidateSmokeCheck{
			Service: "store", Method: "POST", URL: "http://" + storeTunnelListen + "/checkout", ExpectedStatus: 200,
		},
		PaywallValidation: profile.ReleaseCandidateSmokeCheck{
			Service: "paywall", Method: "POST", URL: "http://" + paywallListen + "/pay", ExpectedStatus: 200,
		},
		GracefulRestart: profile.ReleaseCandidateRestartPlan{
			PostRestartChecks: healthChecks,
		},
	}
}
