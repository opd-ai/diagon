package profile

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDebianPackagePlanSuccess(t *testing.T) {
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
		Name:                "local-single-host-bootstrap",
		ServiceContractFile: "service-contract.json",
		StartupSequence:     []string{"i2pd", "paywall", "store", "diagonctl"},
		ExpectedTunnels:     []string{"store-http", "paywall-http"},
		Components: []BootstrapComponent{
			{Name: "i2pd", Listen: "127.0.0.1:7070", HealthURL: "http://127.0.0.1:7070/health", ConfigPath: "/etc/diagon/i2pd/i2pd.conf", Settings: map[string]string{"tunnel_config_path": "/etc/diagon/i2pd/tunnels.conf"}},
			{Name: "paywall", Listen: "127.0.0.1:8081", HealthURL: "http://127.0.0.1:8081/healthz", ConfigPath: "/etc/diagon/paywall/config.yaml", WalletMode: "stubbed", SecretRefs: []string{"PAYWALL_TOKEN"}, Settings: map[string]string{"wallet_rpc_url": "http://127.0.0.1:18089/json_rpc"}},
			{Name: "store", Listen: "127.0.0.1:8080", HealthURL: "http://127.0.0.1:8080/healthz", ConfigPath: "/etc/diagon/store/config.yaml", SecretRefs: []string{"STORE_SESSION_SECRET"}, Settings: map[string]string{"paywall_endpoint": "http://127.0.0.1:8081/api/v1/payments"}},
		},
		Secrets: []BootstrapSecret{
			{Name: "PAYWALL_TOKEN", Source: "env", Ref: "PAYWALL_TOKEN", Required: true},
			{Name: "STORE_SESSION_SECRET", Source: "file", Ref: "/run/secrets/store-session-secret", Required: true},
		},
	}

	plan, err := BuildDebianPackagePlan(bootstrap, contract)
	if err != nil {
		t.Fatalf("BuildDebianPackagePlan() returned error: %v", err)
	}

	if plan.PackageName != "diagon" {
		t.Fatalf("expected package name diagon, got %q", plan.PackageName)
	}
	if len(plan.ServiceUnits) != 3 {
		t.Fatalf("expected 3 service units, got %d", len(plan.ServiceUnits))
	}
	if !containsPath(plan.Layout.ConfigFiles, "/etc/diagon/i2pd/tunnels.conf") {
		t.Fatal("expected tunnel config file in package layout")
	}
	if !containsPath(plan.Layout.LogDirectories, "/var/log/diagon/store") {
		t.Fatal("expected store log directory in package layout")
	}
	if !containsPath(plan.Layout.StateDirectories, "/var/lib/diagon/paywall") {
		t.Fatal("expected paywall state directory in package layout")
	}
	if got := plan.PostInstall.EnableUnits; len(got) != 3 || got[0] != "diagon-i2pd.service" || got[2] != "diagon-store.service" {
		t.Fatalf("unexpected enable order: %v", got)
	}
	if got := plan.Uninstall.StopUnits; len(got) != 3 || got[0] != "diagon-store.service" || got[2] != "diagon-i2pd.service" {
		t.Fatalf("unexpected uninstall stop order: %v", got)
	}

	storeUnit := plan.ServiceUnits[2]
	if !containsPath(storeUnit.After, "diagon-paywall.service") {
		t.Fatalf("expected store unit to start after paywall, got %v", storeUnit.After)
	}
	if !strings.Contains(storeUnit.UnitFile, "ExecStart=/usr/libexec/diagon/store --config /etc/diagon/store/config.yaml") {
		t.Fatalf("expected generated store unit file to contain ExecStart, got:\n%s", storeUnit.UnitFile)
	}
	if !containsValidationCheck(plan.PostInstall.ValidationChecks, "http-2xx", "http://127.0.0.1:8081/healthz") {
		t.Fatal("expected post-install health check for paywall")
	}
	if !containsPath(plan.Uninstall.PreservePaths, "/etc/diagon") {
		t.Fatal("expected uninstall plan to preserve /etc/diagon")
	}
	if !containsPath(plan.Uninstall.RemovePaths, "/run/diagon") {
		t.Fatal("expected uninstall plan to remove /run/diagon")
	}

	for _, binary := range plan.Layout.Binaries {
		if binary.Name == "i2pd" && binary.Source != "debian-dependency" {
			t.Fatalf("expected i2pd binary source to be debian-dependency, got %q", binary.Source)
		}
	}

	for _, configDir := range plan.Layout.ConfigDirectories {
		if !strings.HasPrefix(configDir, "/etc/diagon") {
			t.Fatalf("expected config directory to stay under /etc/diagon, got %q", configDir)
		}
	}
}

func TestBuildDebianPackagePlanServiceOrdering(t *testing.T) {
	t.Parallel()

	plan, err := BuildDebianPackagePlan(loadBootstrapFixture(t), loadServiceContractFixture(t))
	if err != nil {
		t.Fatalf("BuildDebianPackagePlan() returned error: %v", err)
	}

	paywallUnit := plan.ServiceUnits[1]
	if !containsPath(paywallUnit.After, "diagon-i2pd.service") {
		t.Fatalf("expected paywall unit after i2pd, got %v", paywallUnit.After)
	}
	if !containsPath(paywallUnit.Requires, "diagon-i2pd.service") {
		t.Fatalf("expected paywall unit requires i2pd, got %v", paywallUnit.Requires)
	}

	storeUnit := plan.ServiceUnits[2]
	if !containsPath(storeUnit.Requires, "diagon-paywall.service") {
		t.Fatalf("expected store unit to require paywall, got %v", storeUnit.Requires)
	}
	if !strings.Contains(storeUnit.UnitFile, "ReadWritePaths=/etc/diagon/store /run/diagon/store /var/lib/diagon/store /var/log/diagon/store") {
		t.Fatalf("expected store unit file read/write paths, got:\n%s", storeUnit.UnitFile)
	}
	if !containsPath(plan.PostInstall.RequiredFiles, "/etc/diagon/paywall/config.yaml") {
		t.Fatal("expected paywall config in post-install required files")
	}
	if len(plan.PostInstall.SecretRequirements) != 3 {
		t.Fatalf("expected 3 secret requirements, got %d", len(plan.PostInstall.SecretRequirements))
	}
	if !containsPath(plan.Layout.RuntimeDirectories, filepath.Join("/run/diagon", "i2pd")) {
		t.Fatal("expected i2pd runtime directory in layout")
	}
	if len(plan.PostInstall.HealthChecks) != 3 {
		t.Fatalf("expected 3 post-install health checks, got %d", len(plan.PostInstall.HealthChecks))
	}
	if plan.PostInstall.HealthChecks[0].Service != "i2pd" || plan.PostInstall.HealthChecks[2].Service != "store" {
		t.Fatalf("unexpected health check ordering: %+v", plan.PostInstall.HealthChecks)
	}
	if len(plan.Uninstall.Notes) == 0 {
		t.Fatal("expected uninstall guidance notes")
	}
}

func TestWriteDebianPackagePlanRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	err := WriteDebianPackagePlan("", DebianPackagePlan{})
	if err == nil {
		t.Fatal("expected empty output path to fail")
	}
}

func loadBootstrapFixture(t *testing.T) BootstrapProfile {
	t.Helper()

	bootstrap, err := LoadBootstrapProfile(filepath.Join("..", "..", "profiles", "local-single-host-bootstrap.json"))
	if err != nil {
		t.Fatalf("LoadBootstrapProfile() returned error: %v", err)
	}
	return bootstrap
}

func loadServiceContractFixture(t *testing.T) ServiceContract {
	t.Helper()

	contract, err := LoadServiceContract(filepath.Join("..", "..", "profiles", "service-contract.json"))
	if err != nil {
		t.Fatalf("LoadServiceContract() returned error: %v", err)
	}
	return contract
}

func containsPath(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsValidationCheck(checks []DebianValidationCheck, checkType, target string) bool {
	for _, check := range checks {
		if check.Type == checkType && check.Target == target {
			return true
		}
	}
	return false
}
