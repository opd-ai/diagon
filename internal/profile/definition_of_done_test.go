package profile

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildDefinitionOfDoneReportPassesWithLiveEvidence(t *testing.T) {
	matrix, err := LoadIntegrationMatrix(filepath.Join("..", "..", ".github", "integration-matrix.json"))
	if err != nil {
		t.Fatalf("LoadIntegrationMatrix() returned error: %v", err)
	}

	bootstrap, contract := buildDefinitionOfDoneFixtures(t)
	startDefinitionOfDoneStubServices(t, contract)

	releaseDir := t.TempDir()
	for _, file := range []string{"SHA256SUMS", "version-manifest.json", "operator-runbook.md"} {
		path := filepath.Join(releaseDir, "manifest", file)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create manifest directory: %v", err)
		}
		if err := os.WriteFile(path, []byte("ok\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}

	report, err := BuildDefinitionOfDoneReport(bootstrap, contract, matrix, "debian-12", DefinitionOfDoneOptions{
		ProbeLive: true,
		ProbeOptions: RuntimeProbeOptions{
			Timeout:  2 * time.Second,
			Interval: 50 * time.Millisecond,
		},
		ReleaseBundleDir: releaseDir,
	})
	if err != nil {
		t.Fatalf("BuildDefinitionOfDoneReport() returned error: %v", err)
	}

	if !report.ReadyForNextPhase {
		t.Fatalf("expected ReadyForNextPhase=true, got false with report: %+v", report)
	}
	if report.PassedCriteria != 5 || report.PendingCriteria != 0 || report.FailedCriteria != 0 {
		t.Fatalf("unexpected criteria counts: passed=%d pending=%d failed=%d", report.PassedCriteria, report.PendingCriteria, report.FailedCriteria)
	}
}

func TestBuildDefinitionOfDoneReportMarksPendingWhenEvidenceMissing(t *testing.T) {
	t.Parallel()

	matrix, err := LoadIntegrationMatrix(filepath.Join("..", "..", ".github", "integration-matrix.json"))
	if err != nil {
		t.Fatalf("LoadIntegrationMatrix() returned error: %v", err)
	}

	bootstrap, contract := buildDefinitionOfDoneFixtures(t)

	report, err := BuildDefinitionOfDoneReport(bootstrap, contract, matrix, "debian-12", DefinitionOfDoneOptions{})
	if err != nil {
		t.Fatalf("BuildDefinitionOfDoneReport() returned error: %v", err)
	}

	if report.ReadyForNextPhase {
		t.Fatal("expected ReadyForNextPhase=false when live/release evidence is missing")
	}
	if report.PassedCriteria != 2 || report.PendingCriteria != 3 || report.FailedCriteria != 0 {
		t.Fatalf("unexpected criteria counts: passed=%d pending=%d failed=%d", report.PassedCriteria, report.PendingCriteria, report.FailedCriteria)
	}

	if status := findDefinitionOfDoneStatus(t, report, "dod-2-runtime-readiness"); status != DefinitionOfDoneStatusPending {
		t.Fatalf("expected dod-2-runtime-readiness pending, got %q", status)
	}
	if status := findDefinitionOfDoneStatus(t, report, "dod-3-config-and-health"); status != DefinitionOfDoneStatusPending {
		t.Fatalf("expected dod-3-config-and-health pending, got %q", status)
	}
	if status := findDefinitionOfDoneStatus(t, report, "dod-5-release-artifacts"); status != DefinitionOfDoneStatusPending {
		t.Fatalf("expected dod-5-release-artifacts pending, got %q", status)
	}
}

func TestBuildDefinitionOfDoneReportFailsMissingReleaseArtifacts(t *testing.T) {
	t.Parallel()

	matrix, err := LoadIntegrationMatrix(filepath.Join("..", "..", ".github", "integration-matrix.json"))
	if err != nil {
		t.Fatalf("LoadIntegrationMatrix() returned error: %v", err)
	}

	bootstrap, contract := buildDefinitionOfDoneFixtures(t)
	releaseDir := t.TempDir()

	report, err := BuildDefinitionOfDoneReport(bootstrap, contract, matrix, "debian-12", DefinitionOfDoneOptions{ReleaseBundleDir: releaseDir})
	if err != nil {
		t.Fatalf("BuildDefinitionOfDoneReport() returned error: %v", err)
	}

	if status := findDefinitionOfDoneStatus(t, report, "dod-5-release-artifacts"); status != DefinitionOfDoneStatusFailed {
		t.Fatalf("expected dod-5-release-artifacts failed, got %q", status)
	}
}

func TestWriteDefinitionOfDoneReportRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	if err := WriteDefinitionOfDoneReport("", DefinitionOfDoneReport{}); err == nil {
		t.Fatal("expected empty output path to fail")
	}
}

func buildDefinitionOfDoneFixtures(t *testing.T) (BootstrapProfile, ServiceContract) {
	t.Helper()

	i2pdListen := reserveLoopbackAddr(t)
	paywallListen := reserveLoopbackAddr(t)
	storeListen := reserveLoopbackAddr(t)
	storeTunnelListen := reserveLoopbackAddr(t)
	paywallTunnelListen := reserveLoopbackAddr(t)

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{Name: "i2pd", Listen: i2pdListen, HealthURL: fmt.Sprintf("http://%s/health", i2pdListen), StartupOrder: 1},
			{Name: "paywall", Listen: paywallListen, HealthURL: fmt.Sprintf("http://%s/healthz", paywallListen), DependsOn: []string{"i2pd"}, StartupOrder: 2},
			{Name: "store", Listen: storeListen, HealthURL: fmt.Sprintf("http://%s/healthz", storeListen), DependsOn: []string{"i2pd", "paywall"}, StartupOrder: 3},
		},
		APILinks: []APILink{{From: "store", To: "paywall", Endpoint: fmt.Sprintf("http://%s/api/v1/payments", paywallListen)}},
		I2PDTunnels: []I2PDTunnel{
			{Name: "store-http", Type: "http", Listen: storeTunnelListen, Target: storeListen, TargetService: "store"},
			{Name: "paywall-http", Type: "http", Listen: paywallTunnelListen, Target: paywallListen, TargetService: "paywall"},
		},
	}

	bootstrap := BootstrapProfile{
		Name:                "dod-test",
		ServiceContractFile: "profiles/service-contract.json",
		StartupSequence:     []string{"i2pd", "paywall", "store", "diagonctl"},
		ExpectedTunnels:     []string{"store-http", "paywall-http"},
		Components: []BootstrapComponent{
			{Name: "i2pd", Listen: i2pdListen, HealthURL: fmt.Sprintf("http://%s/health", i2pdListen), ConfigPath: "/etc/diagon/i2pd/i2pd.conf", Settings: map[string]string{"tunnel_config_path": "/etc/diagon/i2pd/tunnels.conf"}},
			{Name: "paywall", Listen: paywallListen, HealthURL: fmt.Sprintf("http://%s/healthz", paywallListen), ConfigPath: "/etc/diagon/paywall/config.yaml", WalletMode: "stubbed", SecretRefs: []string{"PAYWALL_WALLET_RPC_USER"}, Settings: map[string]string{"wallet_rpc_url": "http://127.0.0.1:18089/json_rpc", "smoke_payment_path": "/pay"}},
			{Name: "store", Listen: storeListen, HealthURL: fmt.Sprintf("http://%s/healthz", storeListen), ConfigPath: "/etc/diagon/store/config.yaml", SecretRefs: []string{"STORE_SESSION_SECRET"}, Settings: map[string]string{"paywall_endpoint": fmt.Sprintf("http://%s/api/v1/payments", paywallListen), "smoke_checkout_path": "/checkout"}},
		},
		Secrets: []BootstrapSecret{
			{Name: "PAYWALL_WALLET_RPC_USER", Source: "env", Ref: "PAYWALL_WALLET_RPC_USER", Required: true},
			{Name: "STORE_SESSION_SECRET", Source: "file", Ref: "/run/secrets/store-session-secret", Required: true},
		},
	}

	return bootstrap, contract
}

func startDefinitionOfDoneStubServices(t *testing.T, contract ServiceContract) {
	t.Helper()

	serviceByName := make(map[string]ServiceDefinition, len(contract.Services))
	for _, service := range contract.Services {
		serviceByName[service.Name] = service
	}

	for _, name := range []string{"i2pd", "paywall", "store"} {
		service := serviceByName[name]
		listen := service.Listen
		healthPath := "/health"
		if name != "i2pd" {
			healthPath = "/healthz"
		}
		path := healthPath
		stop := startHTTPServerOnAddr(t, listen, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != path {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(stop)
	}

	for _, tunnel := range contract.I2PDTunnels {
		stop := startHTTPServerOnAddr(t, tunnel.Listen, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(stop)
	}
}

func findDefinitionOfDoneStatus(t *testing.T, report DefinitionOfDoneReport, id string) DefinitionOfDoneStatus {
	t.Helper()

	for _, criterion := range report.Criteria {
		if criterion.ID == id {
			return criterion.Status
		}
	}

	t.Fatalf("criterion %q not found", id)
	return ""
}
