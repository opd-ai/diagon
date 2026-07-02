package profile

import (
	"strings"
	"testing"
)

func TestValidateBootstrapProfileDefinitionSuccess(t *testing.T) {
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
		I2PDTunnels: []I2PDTunnel{
			{Name: "store-http", Type: "http", Listen: "127.0.0.1:18080", Target: "127.0.0.1:8080", TargetService: "store"},
			{Name: "paywall-http", Type: "http", Listen: "127.0.0.1:18081", Target: "127.0.0.1:8081", TargetService: "paywall"},
		},
	}

	bootstrap := BootstrapProfile{
		Name:                "local-single-host-bootstrap",
		ServiceContractFile: "profiles/service-contract.json",
		StartupSequence:     []string{"i2pd", "paywall", "store", "diagonctl"},
		ExpectedTunnels:     []string{"store-http", "paywall-http"},
		Components: []BootstrapComponent{
			{
				Name:       "i2pd",
				Listen:     "127.0.0.1:7070",
				HealthURL:  "http://127.0.0.1:7070/health",
				ConfigPath: "/etc/diagon/i2pd/i2pd.conf",
				Settings: map[string]string{
					"tunnel_config_path": "/etc/diagon/i2pd/tunnels.conf",
				},
			},
			{
				Name:       "paywall",
				Listen:     "127.0.0.1:8081",
				HealthURL:  "http://127.0.0.1:8081/healthz",
				ConfigPath: "/etc/diagon/paywall/config.yaml",
				WalletMode: "stubbed",
				SecretRefs: []string{"PAYWALL_WALLET_RPC_USER", "PAYWALL_WALLET_RPC_PASSWORD"},
				Settings: map[string]string{
					"wallet_rpc_url": "http://127.0.0.1:18089/json_rpc",
				},
			},
			{
				Name:                          "store",
				Listen:                        "127.0.0.1:8080",
				HealthURL:                     "http://127.0.0.1:8080/healthz",
				ConfigPath:                    "/etc/diagon/store/config.yaml",
				RequiresExternalI2PDependency: false,
				SecretRefs:                    []string{"STORE_SESSION_SECRET"},
				Settings: map[string]string{
					"paywall_endpoint": "http://127.0.0.1:8081/api/v1/payments",
				},
			},
		},
		Secrets: []BootstrapSecret{
			{Name: "PAYWALL_WALLET_RPC_USER", Source: "env", Ref: "PAYWALL_WALLET_RPC_USER", Required: true},
			{Name: "PAYWALL_WALLET_RPC_PASSWORD", Source: "env", Ref: "PAYWALL_WALLET_RPC_PASSWORD", Required: true},
			{Name: "STORE_SESSION_SECRET", Source: "file", Ref: "/run/secrets/store-session-secret", Required: true},
		},
	}

	result := ValidateBootstrapProfileDefinition(bootstrap, &contract)
	if result.HasErrors() {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
}

func TestValidateBootstrapProfileDefinitionRejectsNonStubbedPaywall(t *testing.T) {
	t.Parallel()

	bootstrap := BootstrapProfile{
		Name:                "local-single-host-bootstrap",
		ServiceContractFile: "profiles/service-contract.json",
		StartupSequence:     []string{"i2pd", "paywall", "store", "diagonctl"},
		ExpectedTunnels:     []string{"store-http", "paywall-http"},
		Components: []BootstrapComponent{
			{
				Name:       "i2pd",
				Listen:     "127.0.0.1:7070",
				HealthURL:  "http://127.0.0.1:7070/health",
				ConfigPath: "/etc/diagon/i2pd/i2pd.conf",
				Settings: map[string]string{
					"tunnel_config_path": "/etc/diagon/i2pd/tunnels.conf",
				},
			},
			{
				Name:       "paywall",
				Listen:     "127.0.0.1:8081",
				HealthURL:  "http://127.0.0.1:8081/healthz",
				ConfigPath: "/etc/diagon/paywall/config.yaml",
				WalletMode: "remote",
				SecretRefs: []string{"PAYWALL_WALLET_RPC_PASSWORD"},
				Settings: map[string]string{
					"wallet_rpc_url": "http://127.0.0.1:18089/json_rpc",
				},
			},
			{
				Name:       "store",
				Listen:     "127.0.0.1:8080",
				HealthURL:  "http://127.0.0.1:8080/healthz",
				ConfigPath: "/etc/diagon/store/config.yaml",
				Settings: map[string]string{
					"paywall_endpoint": "http://127.0.0.1:8081/api/v1/payments",
				},
			},
		},
		Secrets: []BootstrapSecret{{Name: "PAYWALL_WALLET_RPC_PASSWORD", Source: "env", Ref: "PAYWALL_WALLET_RPC_PASSWORD", Required: true}},
	}

	result := ValidateBootstrapProfileDefinition(bootstrap, nil)
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "wallet_mode must be \"stubbed\"") {
		t.Fatalf("expected stubbed wallet validation error, got: %s", joined)
	}
}

func TestValidateBootstrapProfileDefinitionRejectsStoreExternalI2PAndMissingSecret(t *testing.T) {
	t.Parallel()

	bootstrap := BootstrapProfile{
		Name:                "local-single-host-bootstrap",
		ServiceContractFile: "profiles/service-contract.json",
		StartupSequence:     []string{"i2pd", "paywall", "store", "diagonctl"},
		ExpectedTunnels:     []string{"store-http", "paywall-http"},
		Components: []BootstrapComponent{
			{
				Name:       "i2pd",
				Listen:     "127.0.0.1:7070",
				HealthURL:  "http://127.0.0.1:7070/health",
				ConfigPath: "/etc/diagon/i2pd/i2pd.conf",
				Settings: map[string]string{
					"tunnel_config_path": "/etc/diagon/i2pd/tunnels.conf",
				},
			},
			{
				Name:       "paywall",
				Listen:     "127.0.0.1:8081",
				HealthURL:  "http://127.0.0.1:8081/healthz",
				ConfigPath: "/etc/diagon/paywall/config.yaml",
				WalletMode: "stubbed",
				Settings: map[string]string{
					"wallet_rpc_url": "http://127.0.0.1:18089/json_rpc",
				},
			},
			{
				Name:                          "store",
				Listen:                        "127.0.0.1:8080",
				HealthURL:                     "http://127.0.0.1:8080/healthz",
				ConfigPath:                    "/etc/diagon/store/config.yaml",
				RequiresExternalI2PDependency: true,
				SecretRefs:                    []string{"STORE_SESSION_SECRET"},
				Settings: map[string]string{
					"paywall_endpoint": "http://127.0.0.1:8081/api/v1/payments",
				},
			},
		},
	}

	result := ValidateBootstrapProfileDefinition(bootstrap, nil)
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "must not require external I2P dependency") {
		t.Fatalf("expected store external I2P validation error, got: %s", joined)
	}
	if !strings.Contains(joined, "references undefined secret \"STORE_SESSION_SECRET\"") {
		t.Fatalf("expected missing secret validation error, got: %s", joined)
	}
}
