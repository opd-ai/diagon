package profile

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateServiceContractSuccess(t *testing.T) {
	t.Parallel()

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{
				Name:         "i2pd",
				Listen:       "127.0.0.1:7070",
				HealthURL:    "http://127.0.0.1:7070/health",
				StartupOrder: 1,
			},
			{
				Name:         "paywall",
				Listen:       "127.0.0.1:8081",
				HealthURL:    "http://127.0.0.1:8081/healthz",
				DependsOn:    []string{"i2pd"},
				StartupOrder: 2,
			},
			{
				Name:         "store",
				Listen:       "127.0.0.1:8080",
				HealthURL:    "http://127.0.0.1:8080/healthz",
				DependsOn:    []string{"i2pd", "paywall"},
				StartupOrder: 3,
			},
		},
		APILinks: []APILink{
			{
				From:     "store",
				To:       "paywall",
				Endpoint: "http://127.0.0.1:8081/api/v1/payments",
			},
		},
		I2PDTunnels: []I2PDTunnel{
			{
				Name:          "store-http",
				Type:          "http",
				Listen:        "127.0.0.1:18080",
				Target:        "127.0.0.1:8080",
				TargetService: "store",
			},
			{
				Name:          "paywall-http",
				Type:          "http",
				Listen:        "127.0.0.1:18081",
				Target:        "127.0.0.1:8081",
				TargetService: "paywall",
			},
		},
	}

	result := ValidateServiceContractDefinition(contract)
	if result.HasErrors() {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
}

func TestValidateServiceContractTunnelTargetPortMismatch(t *testing.T) {
	t.Parallel()

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{
				Name:         "i2pd",
				Listen:       "127.0.0.1:7070",
				HealthURL:    "http://127.0.0.1:7070/health",
				StartupOrder: 1,
			},
			{
				Name:         "paywall",
				Listen:       "127.0.0.1:8081",
				HealthURL:    "http://127.0.0.1:8081/healthz",
				DependsOn:    []string{"i2pd"},
				StartupOrder: 2,
			},
			{
				Name:         "store",
				Listen:       "127.0.0.1:8080",
				HealthURL:    "http://127.0.0.1:8080/healthz",
				DependsOn:    []string{"i2pd", "paywall"},
				StartupOrder: 3,
			},
		},
		APILinks: []APILink{
			{
				From:     "store",
				To:       "paywall",
				Endpoint: "http://127.0.0.1:8081/api/v1/payments",
			},
		},
		I2PDTunnels: []I2PDTunnel{
			{
				Name:          "store-http",
				Type:          "http",
				Listen:        "127.0.0.1:18080",
				Target:        "127.0.0.1:9999",
				TargetService: "store",
			},
			{
				Name:          "paywall-http",
				Type:          "http",
				Listen:        "127.0.0.1:18081",
				Target:        "127.0.0.1:8081",
				TargetService: "paywall",
			},
		},
	}

	result := ValidateServiceContractDefinition(contract)
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "target port 9999 must match target_service \"store\" listen port 8080") {
		t.Fatalf("expected tunnel target port compatibility error, got: %s", joined)
	}
}

func TestValidateServiceContractMissingTunnelMapping(t *testing.T) {
	t.Parallel()

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{
				Name:         "i2pd",
				Listen:       "127.0.0.1:7070",
				HealthURL:    "http://127.0.0.1:7070/health",
				StartupOrder: 1,
			},
			{
				Name:         "paywall",
				Listen:       "127.0.0.1:8081",
				HealthURL:    "http://127.0.0.1:8081/healthz",
				DependsOn:    []string{"i2pd"},
				StartupOrder: 2,
			},
			{
				Name:         "store",
				Listen:       "127.0.0.1:8080",
				HealthURL:    "http://127.0.0.1:8080/healthz",
				DependsOn:    []string{"i2pd", "paywall"},
				StartupOrder: 3,
			},
		},
		APILinks: []APILink{
			{
				From:     "store",
				To:       "paywall",
				Endpoint: "http://127.0.0.1:8081/api/v1/payments",
			},
		},
		I2PDTunnels: []I2PDTunnel{
			{
				Name:          "store-http",
				Type:          "http",
				Listen:        "127.0.0.1:18080",
				Target:        "127.0.0.1:8080",
				TargetService: "store",
			},
		},
	}

	result := ValidateServiceContractDefinition(contract)
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "service \"paywall\" must have at least one i2pd tunnel mapping") {
		t.Fatalf("expected missing paywall tunnel mapping error, got: %s", joined)
	}
}

func TestValidateServiceContractMissingRequiredService(t *testing.T) {
	t.Parallel()

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{
				Name:         "i2pd",
				Listen:       "127.0.0.1:7070",
				HealthURL:    "http://127.0.0.1:7070/health",
				StartupOrder: 1,
			},
			{
				Name:         "store",
				Listen:       "127.0.0.1:8080",
				HealthURL:    "http://127.0.0.1:8080/healthz",
				DependsOn:    []string{"i2pd"},
				StartupOrder: 2,
			},
		},
	}

	result := ValidateServiceContractDefinition(contract)
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "missing required service \"paywall\"") {
		t.Fatalf("expected missing paywall error, got: %s", joined)
	}
}

func TestValidateServiceContractAPILinkPortMismatch(t *testing.T) {
	t.Parallel()

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{
				Name:         "i2pd",
				Listen:       "127.0.0.1:7070",
				HealthURL:    "http://127.0.0.1:7070/health",
				StartupOrder: 1,
			},
			{
				Name:         "paywall",
				Listen:       "127.0.0.1:8081",
				HealthURL:    "http://127.0.0.1:8081/healthz",
				DependsOn:    []string{"i2pd"},
				StartupOrder: 2,
			},
			{
				Name:         "store",
				Listen:       "127.0.0.1:8080",
				HealthURL:    "http://127.0.0.1:8080/healthz",
				DependsOn:    []string{"i2pd", "paywall"},
				StartupOrder: 3,
			},
		},
		APILinks: []APILink{
			{
				From:     "store",
				To:       "paywall",
				Endpoint: "http://127.0.0.1:9099/api/v1/payments",
			},
		},
	}

	result := ValidateServiceContractDefinition(contract)
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "endpoint port 9099 must match target listen port 8081") {
		t.Fatalf("expected endpoint compatibility error, got: %s", joined)
	}
}

func TestValidateServiceContractDependencyCycle(t *testing.T) {
	t.Parallel()

	contract := ServiceContract{
		Services: []ServiceDefinition{
			{
				Name:         "i2pd",
				Listen:       "127.0.0.1:7070",
				HealthURL:    "http://127.0.0.1:7070/health",
				DependsOn:    []string{"store"},
				StartupOrder: 1,
			},
			{
				Name:         "paywall",
				Listen:       "127.0.0.1:8081",
				HealthURL:    "http://127.0.0.1:8081/healthz",
				DependsOn:    []string{"i2pd"},
				StartupOrder: 2,
			},
			{
				Name:         "store",
				Listen:       "127.0.0.1:8080",
				HealthURL:    "http://127.0.0.1:8080/healthz",
				DependsOn:    []string{"paywall"},
				StartupOrder: 3,
			},
		},
	}

	result := ValidateServiceContractDefinition(contract)
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "dependency cycle detected") {
		t.Fatalf("expected cycle detection error, got: %s", joined)
	}
}

func TestLoadServiceContractSuccess(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "service-contract.json")
	raw := map[string]any{
		"services": []map[string]any{
			{
				"name":          "i2pd",
				"listen":        "127.0.0.1:7070",
				"health_url":    "http://127.0.0.1:7070/health",
				"depends_on":    []string{},
				"startup_order": 1,
			},
		},
	}
	bytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal contract json: %v", err)
	}
	writeFile(t, path, string(bytes))

	contract, err := LoadServiceContract(path)
	if err != nil {
		t.Fatalf("LoadServiceContract() returned error: %v", err)
	}
	if len(contract.Services) != 1 || contract.Services[0].Name != "i2pd" {
		t.Fatalf("unexpected contract contents: %#v", contract)
	}
}
