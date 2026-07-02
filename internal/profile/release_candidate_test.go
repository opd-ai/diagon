package profile

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReleaseCandidateSmokePlanSuccess(t *testing.T) {
	t.Parallel()

	plan, err := BuildReleaseCandidateSmokePlan(loadBootstrapFixture(t), loadServiceContractFixture(t))
	if err != nil {
		t.Fatalf("BuildReleaseCandidateSmokePlan() returned error: %v", err)
	}

	if plan.MarketplaceAccess.URL != "http://127.0.0.1:18080/checkout" {
		t.Fatalf("unexpected marketplace access URL: %q", plan.MarketplaceAccess.URL)
	}
	if plan.PaywallValidation.URL != "http://127.0.0.1:8081/pay" {
		t.Fatalf("unexpected paywall validation URL: %q", plan.PaywallValidation.URL)
	}
	if len(plan.HealthChecks) != 3 {
		t.Fatalf("expected 3 health checks, got %d", len(plan.HealthChecks))
	}
	if len(plan.TunnelEndpoints) != 2 {
		t.Fatalf("expected 2 tunnel endpoints, got %d", len(plan.TunnelEndpoints))
	}
	if got := plan.GracefulRestart.StopUnits; len(got) != 3 || got[0] != "diagon-store.service" || got[2] != "diagon-i2pd.service" {
		t.Fatalf("unexpected graceful restart stop order: %v", got)
	}
	if got := plan.GracefulRestart.StartUnits; len(got) != 3 || got[0] != "diagon-i2pd.service" || got[2] != "diagon-store.service" {
		t.Fatalf("unexpected graceful restart start order: %v", got)
	}
	if plan.WalletMode != "stubbed" {
		t.Fatalf("expected stubbed wallet mode, got %q", plan.WalletMode)
	}
}

func TestBuildOperatorRunbookContainsOperationalSections(t *testing.T) {
	t.Parallel()

	matrix, err := LoadIntegrationMatrix(filepath.Join("..", "..", ".github", "integration-matrix.json"))
	if err != nil {
		t.Fatalf("LoadIntegrationMatrix() returned error: %v", err)
	}
	environment, err := matrix.EnvironmentByName("debian-12")
	if err != nil {
		t.Fatalf("EnvironmentByName() returned error: %v", err)
	}

	runbook, err := BuildOperatorRunbook(loadBootstrapFixture(t), loadServiceContractFixture(t), &environment)
	if err != nil {
		t.Fatalf("BuildOperatorRunbook() returned error: %v", err)
	}

	for _, needle := range []string{
		"# Diagon Operator Runbook",
		"## Start",
		"sudo systemctl start diagon-i2pd.service",
		"## Stop",
		"sudo systemctl stop diagon-store.service",
		"## Status",
		"curl -fsS http://127.0.0.1:7070/health",
		"## Logs",
		"journalctl -u diagon-paywall.service -n 200 --no-pager",
		"## Recovery",
		"curl -fsS -X POST http://127.0.0.1:18080/checkout",
	} {
		if !strings.Contains(runbook, needle) {
			t.Fatalf("expected runbook to contain %q, got:\n%s", needle, runbook)
		}
	}
}

func TestBuildWalletValidationChecklistContainsProductionChecks(t *testing.T) {
	t.Parallel()

	matrix, err := LoadIntegrationMatrix(filepath.Join("..", "..", ".github", "integration-matrix.json"))
	if err != nil {
		t.Fatalf("LoadIntegrationMatrix() returned error: %v", err)
	}
	environment, err := matrix.EnvironmentByName("debian-12")
	if err != nil {
		t.Fatalf("EnvironmentByName() returned error: %v", err)
	}

	checklist, err := BuildWalletValidationChecklist(loadBootstrapFixture(t), loadServiceContractFixture(t), &environment)
	if err != nil {
		t.Fatalf("BuildWalletValidationChecklist() returned error: %v", err)
	}

	for _, needle := range []string{
		"# Diagon Production Wallet Validation Checklist",
		"wallet_mode: stubbed",
		"http://127.0.0.1:18089/json_rpc",
		"curl -fsS -X POST http://127.0.0.1:18089/json_rpc",
		"curl -fsS http://127.0.0.1:8081/healthz",
		"PAYWALL_WALLET_RPC_USER",
		"PAYWALL_WALLET_RPC_PASSWORD",
		"## Success Criteria",
	} {
		if !strings.Contains(checklist, needle) {
			t.Fatalf("expected wallet checklist to contain %q, got:\n%s", needle, checklist)
		}
	}
}

func TestBuildBootstrapQuickstartGuideContainsSecretsAndCommands(t *testing.T) {
	t.Parallel()

	matrix, err := LoadIntegrationMatrix(filepath.Join("..", "..", ".github", "integration-matrix.json"))
	if err != nil {
		t.Fatalf("LoadIntegrationMatrix() returned error: %v", err)
	}
	environment, err := matrix.EnvironmentByName("debian-12")
	if err != nil {
		t.Fatalf("EnvironmentByName() returned error: %v", err)
	}

	guide, err := BuildBootstrapQuickstartGuide(loadBootstrapFixture(t), loadServiceContractFixture(t), &environment)
	if err != nil {
		t.Fatalf("BuildBootstrapQuickstartGuide() returned error: %v", err)
	}

	for _, needle := range []string{
		"# Diagon Single-Host Bootstrap Quickstart",
		"--bootstrap-profile-file profiles/local-single-host-bootstrap.json",
		"--service-contract-file service-contract.json",
		"export PAYWALL_WALLET_RPC_USER='<redacted>'",
		"install -m 600 /dev/null /run/secrets/store-session-secret",
		"--probe-live",
		"aggregated_health.ready=true",
	} {
		if !strings.Contains(guide, needle) {
			t.Fatalf("expected quickstart guide to contain %q, got:\n%s", needle, guide)
		}
	}
}

func TestBuildReleaseCandidateBaselineFromMatrix(t *testing.T) {
	t.Parallel()

	matrix, err := LoadIntegrationMatrix(filepath.Join("..", "..", ".github", "integration-matrix.json"))
	if err != nil {
		t.Fatalf("LoadIntegrationMatrix() returned error: %v", err)
	}

	baseline, err := BuildReleaseCandidateBaseline(matrix, "debian-12")
	if err != nil {
		t.Fatalf("BuildReleaseCandidateBaseline() returned error: %v", err)
	}

	if baseline.TagName != "integration-debian-12-v2026.07.02" {
		t.Fatalf("unexpected tag name: %q", baseline.TagName)
	}
	if baseline.Components["store"].Version != "v0.9.3" {
		t.Fatalf("unexpected store version: %q", baseline.Components["store"].Version)
	}
	if len(baseline.ContractFixtures) != 3 {
		t.Fatalf("expected 3 contract fixtures, got %d", len(baseline.ContractFixtures))
	}
	if !containsPath(baseline.PackageDependencies, "simple-cdd") {
		t.Fatal("expected simple-cdd package dependency in baseline")
	}
	if len(baseline.QualityGates) != 2 {
		t.Fatalf("expected 2 quality gates, got %d", len(baseline.QualityGates))
	}
}

func TestWriteReleaseCandidateArtifactsRejectEmptyPath(t *testing.T) {
	t.Parallel()

	if err := WriteReleaseCandidateSmokePlan("", ReleaseCandidateSmokePlan{}); err == nil {
		t.Fatal("expected empty smoke plan output path to fail")
	}
	if err := WriteOperatorRunbook("", "runbook"); err == nil {
		t.Fatal("expected empty runbook output path to fail")
	}
	if err := WriteReleaseCandidateBaseline("", ReleaseCandidateBaseline{}); err == nil {
		t.Fatal("expected empty baseline output path to fail")
	}
	if err := WriteWalletValidationChecklist("", "checklist"); err == nil {
		t.Fatal("expected empty wallet checklist output path to fail")
	}
	if err := WriteBootstrapQuickstartGuide("", "quickstart"); err == nil {
		t.Fatal("expected empty bootstrap quickstart output path to fail")
	}
}
