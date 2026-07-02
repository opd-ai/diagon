package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const orchestrationCheckName = "diagonctl"

type BootstrapProfile struct {
	Name                string               `json:"name"`
	ServiceContractFile string               `json:"service_contract_file"`
	StartupSequence     []string             `json:"startup_sequence"`
	ExpectedTunnels     []string             `json:"expected_tunnels"`
	Components          []BootstrapComponent `json:"components"`
	Secrets             []BootstrapSecret    `json:"secrets"`
}

type BootstrapComponent struct {
	Name                          string            `json:"name"`
	Listen                        string            `json:"listen"`
	HealthURL                     string            `json:"health_url"`
	ConfigPath                    string            `json:"config_path"`
	WalletMode                    string            `json:"wallet_mode,omitempty"`
	RequiresExternalI2PDependency bool              `json:"requires_external_i2p_dependency,omitempty"`
	SecretRefs                    []string          `json:"secret_refs,omitempty"`
	Settings                      map[string]string `json:"settings,omitempty"`
}

type BootstrapSecret struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Ref      string `json:"ref"`
	Required bool   `json:"required"`
}

func LoadBootstrapProfile(path string) (BootstrapProfile, error) {
	if strings.TrimSpace(path) == "" {
		return BootstrapProfile{}, errors.New("bootstrap profile path cannot be empty")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return BootstrapProfile{}, fmt.Errorf("read bootstrap profile file %s: %w", path, err)
	}

	var profile BootstrapProfile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return BootstrapProfile{}, fmt.Errorf("parse bootstrap profile file %s: %w", path, err)
	}

	if len(profile.Components) == 0 {
		return BootstrapProfile{}, fmt.Errorf("bootstrap profile file %s must include at least one component", path)
	}

	return profile, nil
}

func ValidateBootstrapProfile(path string, contract *ServiceContract) (Result, error) {
	bootstrap, err := LoadBootstrapProfile(path)
	if err != nil {
		return Result{}, err
	}

	return ValidateBootstrapProfileDefinition(bootstrap, contract), nil
}

func ValidateBootstrapProfileDefinition(bootstrap BootstrapProfile, contract *ServiceContract) Result {
	result := Result{}

	if strings.TrimSpace(bootstrap.Name) == "" {
		result.Errors = append(result.Errors, "bootstrap profile name cannot be empty")
	}
	if strings.TrimSpace(bootstrap.ServiceContractFile) == "" {
		result.Errors = append(result.Errors, "bootstrap profile must declare service_contract_file")
	}

	if len(bootstrap.Components) == 0 {
		result.Errors = append(result.Errors, "bootstrap profile must define at least one component")
		result.Sort()
		return result
	}

	secretByName := make(map[string]BootstrapSecret, len(bootstrap.Secrets))
	for _, secret := range bootstrap.Secrets {
		name := strings.TrimSpace(secret.Name)
		if name == "" {
			result.Errors = append(result.Errors, "bootstrap profile contains secret with empty name")
			continue
		}
		if _, exists := secretByName[name]; exists {
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate bootstrap secret %q", name))
			continue
		}
		source := strings.TrimSpace(strings.ToLower(secret.Source))
		if source != "env" && source != "file" {
			result.Errors = append(result.Errors, fmt.Sprintf("bootstrap secret %q has unsupported source %q", name, secret.Source))
		}
		if strings.TrimSpace(secret.Ref) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("bootstrap secret %q must include non-empty ref", name))
		}
		if source == "file" && !filepath.IsAbs(strings.TrimSpace(secret.Ref)) {
			result.Errors = append(result.Errors, fmt.Sprintf("bootstrap secret %q file ref %q must be an absolute path", name, secret.Ref))
		}
		secretByName[name] = secret
	}

	componentByName := make(map[string]BootstrapComponent, len(bootstrap.Components))
	for _, component := range bootstrap.Components {
		name := strings.TrimSpace(component.Name)
		if name == "" {
			result.Errors = append(result.Errors, "bootstrap profile contains component with empty name")
			continue
		}
		if _, exists := componentByName[name]; exists {
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate bootstrap component %q", name))
			continue
		}

		host, port, listenErr := splitHostPort(component.Listen)
		if listenErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q has invalid listen address %q: %v", name, component.Listen, listenErr))
		} else if !isLoopbackHost(host) {
			result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q listen host %q must be local-only", name, host))
		}

		healthHost, healthPort, healthErr := validateServiceURL(component.HealthURL)
		if healthErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q has invalid health_url %q: %v", name, component.HealthURL, healthErr))
		} else {
			if !isLoopbackHost(healthHost) {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q health_url host %q must be local-only", name, healthHost))
			}
			if port != 0 && healthPort != 0 && port != healthPort {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q listen port %d must match health_url port %d", name, port, healthPort))
			}
		}

		configPath := strings.TrimSpace(component.ConfigPath)
		if configPath == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q must declare config_path", name))
		} else if !filepath.IsAbs(configPath) {
			result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q config_path %q must be an absolute path", name, component.ConfigPath))
		}

		for _, secretRef := range component.SecretRefs {
			trimmedRef := strings.TrimSpace(secretRef)
			if trimmedRef == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q contains empty secret ref", name))
				continue
			}
			if _, exists := secretByName[trimmedRef]; !exists {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q references undefined secret %q", name, trimmedRef))
			}
		}

		switch name {
		case requiredServiceI2PD:
			tunnelConfigPath := strings.TrimSpace(component.Settings["tunnel_config_path"])
			if tunnelConfigPath == "" {
				result.Errors = append(result.Errors, "bootstrap component \"i2pd\" must define settings.tunnel_config_path")
			} else if !filepath.IsAbs(tunnelConfigPath) {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component \"i2pd\" tunnel_config_path %q must be an absolute path", tunnelConfigPath))
			}
		case requiredServicePaywall:
			if strings.TrimSpace(strings.ToLower(component.WalletMode)) != "stubbed" {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q wallet_mode must be %q", name, "stubbed"))
			}
			walletRPCURL := strings.TrimSpace(component.Settings["wallet_rpc_url"])
			if walletRPCURL == "" {
				result.Errors = append(result.Errors, "bootstrap component \"paywall\" must define settings.wallet_rpc_url")
			} else if rpcHost, _, rpcErr := validateServiceURL(walletRPCURL); rpcErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component \"paywall\" wallet_rpc_url %q is invalid: %v", walletRPCURL, rpcErr))
			} else if !isLoopbackHost(rpcHost) {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component \"paywall\" wallet_rpc_url host %q must be local-only", rpcHost))
			}
		case requiredServiceStore:
			if component.RequiresExternalI2PDependency {
				result.Errors = append(result.Errors, "bootstrap component \"store\" must not require external I2P dependency")
			}
			paywallEndpoint := strings.TrimSpace(component.Settings["paywall_endpoint"])
			if paywallEndpoint == "" {
				result.Errors = append(result.Errors, "bootstrap component \"store\" must define settings.paywall_endpoint")
			} else if endpointHost, _, endpointErr := validateServiceURL(paywallEndpoint); endpointErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component \"store\" paywall_endpoint %q is invalid: %v", paywallEndpoint, endpointErr))
			} else if !isLoopbackHost(endpointHost) {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component \"store\" paywall_endpoint host %q must be local-only", endpointHost))
			}
		}

		componentByName[name] = component
	}

	for _, required := range []string{requiredServiceI2PD, requiredServicePaywall, requiredServiceStore} {
		if _, exists := componentByName[required]; !exists {
			result.Errors = append(result.Errors, fmt.Sprintf("bootstrap profile missing required component %q", required))
		}
	}

	expectedSequence := expectedBootstrapStartupSequence(contract)
	if len(bootstrap.StartupSequence) != len(expectedSequence) {
		result.Errors = append(result.Errors, fmt.Sprintf("bootstrap startup_sequence must be %v", expectedSequence))
	} else {
		for idx, item := range expectedSequence {
			if strings.TrimSpace(bootstrap.StartupSequence[idx]) != item {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap startup_sequence must be %v", expectedSequence))
				break
			}
		}
	}

	if len(bootstrap.ExpectedTunnels) == 0 {
		result.Errors = append(result.Errors, "bootstrap profile must define expected_tunnels")
	}

	if contract != nil {
		serviceByName := make(map[string]ServiceDefinition, len(contract.Services))
		for _, service := range contract.Services {
			serviceByName[service.Name] = service
		}

		for name, component := range componentByName {
			service, exists := serviceByName[name]
			if !exists {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q has no matching service contract entry", name))
				continue
			}
			if strings.TrimSpace(component.Listen) != strings.TrimSpace(service.Listen) {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q listen %q must match service contract listen %q", name, component.Listen, service.Listen))
			}
			if strings.TrimSpace(component.HealthURL) != strings.TrimSpace(service.HealthURL) {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap component %q health_url %q must match service contract health_url %q", name, component.HealthURL, service.HealthURL))
			}
		}

		tunnelByName := make(map[string]I2PDTunnel, len(contract.I2PDTunnels))
		for _, tunnel := range contract.I2PDTunnels {
			tunnelByName[tunnel.Name] = tunnel
		}
		for _, tunnelName := range bootstrap.ExpectedTunnels {
			trimmedTunnelName := strings.TrimSpace(tunnelName)
			if trimmedTunnelName == "" {
				result.Errors = append(result.Errors, "bootstrap expected_tunnels contains empty tunnel name")
				continue
			}
			if _, exists := tunnelByName[trimmedTunnelName]; !exists {
				result.Errors = append(result.Errors, fmt.Sprintf("bootstrap expected tunnel %q is missing from service contract", trimmedTunnelName))
			}
		}
	}

	result.Sort()
	return result
}

func expectedBootstrapStartupSequence(contract *ServiceContract) []string {
	if contract == nil || len(contract.Services) == 0 {
		return []string{requiredServiceI2PD, requiredServicePaywall, requiredServiceStore, orchestrationCheckName}
	}

	services := append([]ServiceDefinition{}, contract.Services...)
	sort.Slice(services, func(i, j int) bool {
		if services[i].StartupOrder == services[j].StartupOrder {
			return services[i].Name < services[j].Name
		}
		return services[i].StartupOrder < services[j].StartupOrder
	})

	sequence := make([]string, 0, len(services)+1)
	for _, service := range services {
		sequence = append(sequence, strings.TrimSpace(service.Name))
	}
	sequence = append(sequence, orchestrationCheckName)
	return sequence
}
