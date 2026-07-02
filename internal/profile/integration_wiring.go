package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type InjectedConfigBundle struct {
	Store   InjectedStoreConfig   `json:"store"`
	Paywall InjectedPaywallConfig `json:"paywall"`
	I2PD    InjectedI2PDConfig    `json:"i2pd"`
}

type InjectedStoreConfig struct {
	ConfigPath                    string            `json:"config_path"`
	ListenHost                    string            `json:"listen_host"`
	ListenPort                    int               `json:"listen_port"`
	HealthURL                     string            `json:"health_url"`
	PaywallEndpoint               string            `json:"paywall_endpoint"`
	RequiresExternalI2PDependency bool              `json:"requires_external_i2p_dependency"`
	SecretRefs                    []string          `json:"secret_refs,omitempty"`
	I2PInboundEndpoints           []string          `json:"i2p_inbound_endpoints,omitempty"`
	I2PPaywallRouteEndpoints      []string          `json:"i2p_paywall_route_endpoints,omitempty"`
	AdditionalSettings            map[string]string `json:"additional_settings,omitempty"`
}

type InjectedPaywallConfig struct {
	ConfigPath         string            `json:"config_path"`
	ListenHost         string            `json:"listen_host"`
	ListenPort         int               `json:"listen_port"`
	HealthURL          string            `json:"health_url"`
	WalletMode         string            `json:"wallet_mode"`
	WalletRPCURL       string            `json:"wallet_rpc_url"`
	SecretRefs         []string          `json:"secret_refs,omitempty"`
	AdditionalSettings map[string]string `json:"additional_settings,omitempty"`
}

type InjectedI2PDConfig struct {
	ConfigPath       string               `json:"config_path"`
	TunnelConfigPath string               `json:"tunnel_config_path"`
	ListenHost       string               `json:"listen_host"`
	ListenPort       int                  `json:"listen_port"`
	HealthURL        string               `json:"health_url"`
	Tunnels          []InjectedI2PDTunnel `json:"tunnels"`
	AdditionalConfig map[string]string    `json:"additional_config,omitempty"`
	StartupSequence  []string             `json:"startup_sequence"`
	ExpectedTunnels  []string             `json:"expected_tunnels"`
}

type InjectedI2PDTunnel struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Listen        string `json:"listen"`
	Target        string `json:"target"`
	TargetService string `json:"target_service"`
}

func BuildInjectedConfigBundle(bootstrap BootstrapProfile, contract ServiceContract) (InjectedConfigBundle, error) {
	contractResult := ValidateServiceContractDefinition(contract)
	if contractResult.HasErrors() {
		return InjectedConfigBundle{}, fmt.Errorf("invalid service contract: %s", strings.Join(contractResult.Errors, "; "))
	}

	bootstrapResult := ValidateBootstrapProfileDefinition(bootstrap, &contract)
	if bootstrapResult.HasErrors() {
		return InjectedConfigBundle{}, fmt.Errorf("invalid bootstrap profile: %s", strings.Join(bootstrapResult.Errors, "; "))
	}

	components := make(map[string]BootstrapComponent, len(bootstrap.Components))
	for _, component := range bootstrap.Components {
		components[component.Name] = component
	}

	i2pdComponent, hasI2PD := components[requiredServiceI2PD]
	paywallComponent, hasPaywall := components[requiredServicePaywall]
	storeComponent, hasStore := components[requiredServiceStore]
	if !hasI2PD || !hasPaywall || !hasStore {
		return InjectedConfigBundle{}, fmt.Errorf("bootstrap profile must define %q, %q, and %q components", requiredServiceI2PD, requiredServicePaywall, requiredServiceStore)
	}

	storePaywallEndpoint := strings.TrimSpace(storeComponent.Settings["paywall_endpoint"])
	apiPaywallEndpoint, hasAPILink := paywallLinkEndpoint(contract)
	if hasAPILink && !sameHostPortURL(storePaywallEndpoint, apiPaywallEndpoint) {
		return InjectedConfigBundle{}, fmt.Errorf("bootstrap store paywall_endpoint %q must match service contract api link endpoint %q host/port", storePaywallEndpoint, apiPaywallEndpoint)
	}

	storeHost, storePort, err := splitHostPort(storeComponent.Listen)
	if err != nil {
		return InjectedConfigBundle{}, fmt.Errorf("parse store listen %q: %w", storeComponent.Listen, err)
	}
	paywallHost, paywallPort, err := splitHostPort(paywallComponent.Listen)
	if err != nil {
		return InjectedConfigBundle{}, fmt.Errorf("parse paywall listen %q: %w", paywallComponent.Listen, err)
	}
	i2pdHost, i2pdPort, err := splitHostPort(i2pdComponent.Listen)
	if err != nil {
		return InjectedConfigBundle{}, fmt.Errorf("parse i2pd listen %q: %w", i2pdComponent.Listen, err)
	}

	storeInboundEndpoints := make([]string, 0)
	paywallRouteEndpoints := make([]string, 0)
	injectedTunnels := make([]InjectedI2PDTunnel, 0, len(contract.I2PDTunnels))
	for _, tunnel := range contract.I2PDTunnels {
		injectedTunnels = append(injectedTunnels, InjectedI2PDTunnel{
			Name:          tunnel.Name,
			Type:          tunnel.Type,
			Listen:        tunnel.Listen,
			Target:        tunnel.Target,
			TargetService: tunnel.TargetService,
		})
		switch strings.TrimSpace(tunnel.TargetService) {
		case requiredServiceStore:
			storeInboundEndpoints = append(storeInboundEndpoints, asHTTPURL(tunnel.Listen))
		case requiredServicePaywall:
			paywallRouteEndpoints = append(paywallRouteEndpoints, asHTTPURL(tunnel.Listen))
		}
	}
	sort.Strings(storeInboundEndpoints)
	sort.Strings(paywallRouteEndpoints)

	bundle := InjectedConfigBundle{
		Store: InjectedStoreConfig{
			ConfigPath:                    strings.TrimSpace(storeComponent.ConfigPath),
			ListenHost:                    storeHost,
			ListenPort:                    storePort,
			HealthURL:                     strings.TrimSpace(storeComponent.HealthURL),
			PaywallEndpoint:               storePaywallEndpoint,
			RequiresExternalI2PDependency: storeComponent.RequiresExternalI2PDependency,
			SecretRefs:                    append([]string{}, storeComponent.SecretRefs...),
			I2PInboundEndpoints:           storeInboundEndpoints,
			I2PPaywallRouteEndpoints:      paywallRouteEndpoints,
			AdditionalSettings:            copySettings(storeComponent.Settings, "paywall_endpoint"),
		},
		Paywall: InjectedPaywallConfig{
			ConfigPath:         strings.TrimSpace(paywallComponent.ConfigPath),
			ListenHost:         paywallHost,
			ListenPort:         paywallPort,
			HealthURL:          strings.TrimSpace(paywallComponent.HealthURL),
			WalletMode:         strings.TrimSpace(paywallComponent.WalletMode),
			WalletRPCURL:       strings.TrimSpace(paywallComponent.Settings["wallet_rpc_url"]),
			SecretRefs:         append([]string{}, paywallComponent.SecretRefs...),
			AdditionalSettings: copySettings(paywallComponent.Settings, "wallet_rpc_url"),
		},
		I2PD: InjectedI2PDConfig{
			ConfigPath:       strings.TrimSpace(i2pdComponent.ConfigPath),
			TunnelConfigPath: strings.TrimSpace(i2pdComponent.Settings["tunnel_config_path"]),
			ListenHost:       i2pdHost,
			ListenPort:       i2pdPort,
			HealthURL:        strings.TrimSpace(i2pdComponent.HealthURL),
			Tunnels:          injectedTunnels,
			AdditionalConfig: copySettings(i2pdComponent.Settings, "tunnel_config_path"),
			StartupSequence:  append([]string{}, bootstrap.StartupSequence...),
			ExpectedTunnels:  append([]string{}, bootstrap.ExpectedTunnels...),
		},
	}

	return bundle, nil
}

func WriteInjectedConfigBundle(path string, bundle InjectedConfigBundle) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("injected config output path cannot be empty")
	}

	raw, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal injected config bundle: %w", err)
	}
	raw = append(raw, '\n')

	if trimmedPath == "-" {
		if _, err := os.Stdout.Write(raw); err != nil {
			return fmt.Errorf("write injected config bundle to stdout: %w", err)
		}
		return nil
	}

	if err := os.WriteFile(trimmedPath, raw, 0o644); err != nil {
		return fmt.Errorf("write injected config bundle to %s: %w", trimmedPath, err)
	}
	return nil
}

func copySettings(source map[string]string, excludedKeys ...string) map[string]string {
	if len(source) == 0 {
		return nil
	}

	excluded := make(map[string]struct{}, len(excludedKeys))
	for _, key := range excludedKeys {
		excluded[key] = struct{}{}
	}

	copy := make(map[string]string)
	for key, value := range source {
		if _, skip := excluded[key]; skip {
			continue
		}
		copy[key] = value
	}

	if len(copy) == 0 {
		return nil
	}
	return copy
}

func paywallLinkEndpoint(contract ServiceContract) (string, bool) {
	for _, link := range contract.APILinks {
		if strings.TrimSpace(link.From) == requiredServiceStore && strings.TrimSpace(link.To) == requiredServicePaywall {
			return strings.TrimSpace(link.Endpoint), true
		}
	}
	return "", false
}

func sameHostPortURL(a, b string) bool {
	aHost, aPort, aErr := validateServiceURL(a)
	bHost, bPort, bErr := validateServiceURL(b)
	if aErr != nil || bErr != nil {
		return false
	}
	return strings.EqualFold(aHost, bHost) && aPort == bPort
}

func asHTTPURL(addr string) string {
	host, port, err := splitHostPort(addr)
	if err != nil {
		return ""
	}
	return "http://" + host + ":" + strconv.Itoa(port)
}
