package profile

import (
	"fmt"
	"strings"
)

const (
	fallbackComposePath       = "/opt/diagon/compose/compose.yaml"
	fallbackEnvTemplatePath   = "/etc/diagon/compose/diagon-compose.env.example"
	fallbackManualGuidePath   = "/opt/diagon/compose/MANUAL-INSTALL.md"
	fallbackSystemdUnitPath   = "/etc/systemd/system/diagon-compose.service"
	fallbackSystemdUnitName   = "diagon-compose.service"
	fallbackSystemdWorkingDir = "/opt/diagon/compose"
)

type DebianComposeServiceBundle struct {
	BundleName           string                         `json:"bundle_name"`
	Environment          string                         `json:"environment,omitempty"`
	DebianCodename       string                         `json:"debian_codename,omitempty"`
	Compose              DebianBundleFile               `json:"compose"`
	EnvironmentTemplate  DebianBundleFile               `json:"environment_template"`
	SystemdUnit          DebianBundleFile               `json:"systemd_unit"`
	ManualInstallGuide   DebianBundleFile               `json:"manual_install_guide"`
	ValidationChecks     []DebianComposeValidationCheck `json:"validation_checks"`
	ManualInstallSteps   []string                       `json:"manual_install_steps"`
	SuccessCriteria      []string                       `json:"success_criteria"`
	PinnedImageReference map[string]string              `json:"pinned_image_reference"`
}

type DebianBundleFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type DebianComposeValidationCheck struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Expect  string `json:"expect"`
}

func BuildDebianComposeServiceBundle(bootstrap BootstrapProfile, contract ServiceContract, environment *IntegrationEnvironment) (DebianComposeServiceBundle, error) {
	contractResult := ValidateServiceContractDefinition(contract)
	if contractResult.HasErrors() {
		return DebianComposeServiceBundle{}, fmt.Errorf("invalid service contract: %s", strings.Join(contractResult.Errors, "; "))
	}

	bootstrapResult := ValidateBootstrapProfileDefinition(bootstrap, &contract)
	if bootstrapResult.HasErrors() {
		return DebianComposeServiceBundle{}, fmt.Errorf("invalid bootstrap profile: %s", strings.Join(bootstrapResult.Errors, "; "))
	}

	componentByName := make(map[string]BootstrapComponent, len(bootstrap.Components))
	for _, component := range bootstrap.Components {
		componentByName[strings.TrimSpace(component.Name)] = component
	}

	i2pdComponent, hasI2PD := componentByName[requiredServiceI2PD]
	paywallComponent, hasPaywall := componentByName[requiredServicePaywall]
	storeComponent, hasStore := componentByName[requiredServiceStore]
	if !hasI2PD || !hasPaywall || !hasStore {
		return DebianComposeServiceBundle{}, fmt.Errorf("bootstrap profile must define %q, %q, and %q components", requiredServiceI2PD, requiredServicePaywall, requiredServiceStore)
	}

	images := map[string]string{
		requiredServiceI2PD:    "ghcr.io/purplei2p/i2pd:latest",
		requiredServicePaywall: "ghcr.io/opd-ai/paywall:latest",
		requiredServiceStore:   "ghcr.io/opd-ai/store:latest",
	}

	bundle := DebianComposeServiceBundle{
		BundleName:           "debian-compose-fallback",
		Compose:              DebianBundleFile{Path: fallbackComposePath},
		EnvironmentTemplate:  DebianBundleFile{Path: fallbackEnvTemplatePath},
		SystemdUnit:          DebianBundleFile{Path: fallbackSystemdUnitPath},
		ManualInstallGuide:   DebianBundleFile{Path: fallbackManualGuidePath},
		PinnedImageReference: map[string]string{},
		ValidationChecks: []DebianComposeValidationCheck{
			{Name: "compose-model", Command: "docker compose -f /opt/diagon/compose/compose.yaml config -q", Expect: "exit code 0"},
			{Name: "stack-active", Command: "systemctl is-active diagon-compose.service", Expect: "active"},
		},
		ManualInstallSteps: []string{
			"Install Docker and Compose plugin on Debian: sudo apt-get update && sudo apt-get install -y docker.io docker-compose-plugin ca-certificates curl",
			"Create runtime directories: sudo install -d -m 755 /opt/diagon/compose /etc/diagon/compose /etc/diagon/i2pd /etc/diagon/paywall /etc/diagon/store /var/lib/diagon /var/log/diagon",
			"Write compose.yaml, diagon-compose.env, and diagon-compose.service to the emitted file paths",
			"Create required secrets such as /run/secrets/store-session-secret and populate PAYWALL_WALLET_RPC_USER/PAYWALL_WALLET_RPC_PASSWORD",
			"Reload systemd and start the stack: sudo systemctl daemon-reload && sudo systemctl enable --now diagon-compose.service",
			"Validate health endpoints and tunnel listeners using the emitted validation checks",
		},
		SuccessCriteria: []string{
			"docker compose config validates the emitted compose bundle",
			"i2pd, paywall, and store services start in dependency order",
			"Store and Paywall local health endpoints return 2xx",
			"i2pd tunnel listeners are reachable on the expected local ports",
		},
	}

	if environment != nil {
		bundle.Environment = strings.TrimSpace(environment.Environment)
		bundle.DebianCodename = strings.TrimSpace(environment.DebianCodename)
		images[requiredServiceI2PD] = imageRefFromMatrixComponent(environment.Components.I2PD)
		images[requiredServicePaywall] = imageRefFromMatrixComponent(environment.Components.Paywall)
		images[requiredServiceStore] = imageRefFromMatrixComponent(environment.Components.Store)
	}

	for name, ref := range images {
		bundle.PinnedImageReference[name] = ref
	}

	tunnelListeners := make([]string, 0, len(contract.I2PDTunnels))
	for _, tunnel := range contract.I2PDTunnels {
		tunnelListeners = append(tunnelListeners, strings.TrimSpace(tunnel.Listen))
		bundle.ValidationChecks = append(bundle.ValidationChecks, DebianComposeValidationCheck{
			Name:    "tunnel-listener-" + strings.TrimSpace(tunnel.Name),
			Command: "nc -z " + strings.TrimSpace(tunnel.Listen),
			Expect:  "listener reachable",
		})
	}

	for _, service := range servicesInStartupOrder(contract.Services) {
		serviceName := strings.TrimSpace(service.Name)
		bundle.ValidationChecks = append(bundle.ValidationChecks, DebianComposeValidationCheck{
			Name:    serviceName + "-health",
			Command: "curl -fsS " + strings.TrimSpace(service.HealthURL),
			Expect:  "2xx",
		})
	}

	bundle.Compose.Content = renderFallbackComposeYAML(
		contract,
		i2pdComponent,
		paywallComponent,
		storeComponent,
		images,
	)
	bundle.EnvironmentTemplate.Content = renderFallbackEnvTemplate(bootstrap, storeComponent, paywallComponent)
	bundle.SystemdUnit.Content = renderFallbackSystemdUnit()
	bundle.ManualInstallGuide.Content = renderFallbackManualGuide(bundle, bootstrap, contract, tunnelListeners)

	return bundle, nil
}

func WriteDebianComposeServiceBundle(path string, bundle DebianComposeServiceBundle) error {
	return writeJSONFile(path, bundle, "debian compose fallback bundle")
}

func imageRefFromMatrixComponent(component IntegrationMatrixComponent) string {
	repo := strings.TrimSpace(component.Repo)
	version := strings.TrimSpace(component.Version)
	if repo == "" || version == "" {
		return ""
	}
	return "ghcr.io/" + repo + ":" + version
}

func renderFallbackComposeYAML(
	contract ServiceContract,
	i2pdComponent BootstrapComponent,
	paywallComponent BootstrapComponent,
	storeComponent BootstrapComponent,
	images map[string]string,
) string {
	var builder strings.Builder
	builder.WriteString("version: \"3.9\"\n")
	builder.WriteString("services:\n")
	builder.WriteString("  i2pd:\n")
	builder.WriteString("    image: ")
	builder.WriteString(images[requiredServiceI2PD])
	builder.WriteString("\n")
	builder.WriteString("    container_name: diagon-i2pd\n")
	builder.WriteString("    restart: unless-stopped\n")
	builder.WriteString("    network_mode: host\n")
	builder.WriteString("    command: [\"/usr/sbin/i2pd\", \"--conf\", \"")
	builder.WriteString(strings.TrimSpace(i2pdComponent.ConfigPath))
	builder.WriteString("\", \"--tunconf\", \"")
	builder.WriteString(strings.TrimSpace(i2pdComponent.Settings["tunnel_config_path"]))
	builder.WriteString("\"]\n")
	builder.WriteString("    volumes:\n")
	builder.WriteString("      - /etc/diagon/i2pd:/etc/diagon/i2pd:rw\n")
	builder.WriteString("      - /var/lib/diagon/i2pd:/var/lib/diagon/i2pd:rw\n")
	builder.WriteString("      - /var/log/diagon/i2pd:/var/log/diagon/i2pd:rw\n")
	builder.WriteString("    healthcheck:\n")
	builder.WriteString("      test: [\"CMD-SHELL\", \"wget -qO- ")
	builder.WriteString(strings.TrimSpace(i2pdComponent.HealthURL))
	builder.WriteString(" >/dev/null 2>&1 || exit 1\"]\n")
	builder.WriteString("      interval: 5s\n")
	builder.WriteString("      timeout: 2s\n")
	builder.WriteString("      retries: 12\n\n")

	builder.WriteString("  paywall:\n")
	builder.WriteString("    image: ")
	builder.WriteString(images[requiredServicePaywall])
	builder.WriteString("\n")
	builder.WriteString("    container_name: diagon-paywall\n")
	builder.WriteString("    restart: unless-stopped\n")
	builder.WriteString("    network_mode: host\n")
	builder.WriteString("    env_file:\n")
	builder.WriteString("      - /etc/diagon/compose/diagon-compose.env\n")
	builder.WriteString("    depends_on:\n")
	builder.WriteString("      i2pd:\n")
	builder.WriteString("        condition: service_healthy\n")
	builder.WriteString("    command: [\"/usr/libexec/diagon/paywall\", \"--config\", \"")
	builder.WriteString(strings.TrimSpace(paywallComponent.ConfigPath))
	builder.WriteString("\"]\n")
	builder.WriteString("    volumes:\n")
	builder.WriteString("      - /etc/diagon/paywall:/etc/diagon/paywall:rw\n")
	builder.WriteString("      - /var/lib/diagon/paywall:/var/lib/diagon/paywall:rw\n")
	builder.WriteString("      - /var/log/diagon/paywall:/var/log/diagon/paywall:rw\n")
	builder.WriteString("    healthcheck:\n")
	builder.WriteString("      test: [\"CMD-SHELL\", \"wget -qO- ")
	builder.WriteString(strings.TrimSpace(paywallComponent.HealthURL))
	builder.WriteString(" >/dev/null 2>&1 || exit 1\"]\n")
	builder.WriteString("      interval: 5s\n")
	builder.WriteString("      timeout: 2s\n")
	builder.WriteString("      retries: 12\n\n")

	builder.WriteString("  store:\n")
	builder.WriteString("    image: ")
	builder.WriteString(images[requiredServiceStore])
	builder.WriteString("\n")
	builder.WriteString("    container_name: diagon-store\n")
	builder.WriteString("    restart: unless-stopped\n")
	builder.WriteString("    network_mode: host\n")
	builder.WriteString("    env_file:\n")
	builder.WriteString("      - /etc/diagon/compose/diagon-compose.env\n")
	builder.WriteString("    depends_on:\n")
	builder.WriteString("      i2pd:\n")
	builder.WriteString("        condition: service_healthy\n")
	builder.WriteString("      paywall:\n")
	builder.WriteString("        condition: service_healthy\n")
	builder.WriteString("    command: [\"/usr/libexec/diagon/store\", \"--config\", \"")
	builder.WriteString(strings.TrimSpace(storeComponent.ConfigPath))
	builder.WriteString("\"]\n")
	builder.WriteString("    volumes:\n")
	builder.WriteString("      - /etc/diagon/store:/etc/diagon/store:rw\n")
	builder.WriteString("      - /var/lib/diagon/store:/var/lib/diagon/store:rw\n")
	builder.WriteString("      - /var/log/diagon/store:/var/log/diagon/store:rw\n")
	builder.WriteString("      - /run/secrets:/run/secrets:ro\n")
	builder.WriteString("    healthcheck:\n")
	builder.WriteString("      test: [\"CMD-SHELL\", \"wget -qO- ")
	builder.WriteString(strings.TrimSpace(storeComponent.HealthURL))
	builder.WriteString(" >/dev/null 2>&1 || exit 1\"]\n")
	builder.WriteString("      interval: 5s\n")
	builder.WriteString("      timeout: 2s\n")
	builder.WriteString("      retries: 12\n")

	builder.WriteString("\n# Startup order contract\n")
	for _, service := range servicesInStartupOrder(contract.Services) {
		builder.WriteString("# - ")
		builder.WriteString(strings.TrimSpace(service.Name))
		builder.WriteString("\n")
	}
	return builder.String()
}

func renderFallbackEnvTemplate(bootstrap BootstrapProfile, storeComponent, paywallComponent BootstrapComponent) string {
	secretByName := make(map[string]BootstrapSecret, len(bootstrap.Secrets))
	for _, secret := range bootstrap.Secrets {
		secretByName[strings.TrimSpace(secret.Name)] = secret
	}

	var builder strings.Builder
	builder.WriteString("# Diagon fallback compose environment template\n")
	builder.WriteString("# Copy to /etc/diagon/compose/diagon-compose.env and fill required values.\n")
	for _, secretRef := range uniqueSortedStrings(paywallComponent.SecretRefs) {
		secret, exists := secretByName[secretRef]
		if !exists {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(secret.Source), "env") {
			builder.WriteString(strings.TrimSpace(secret.Ref))
			builder.WriteString("=<required>\n")
		}
	}

	for _, secretRef := range uniqueSortedStrings(storeComponent.SecretRefs) {
		secret, exists := secretByName[secretRef]
		if !exists {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(secret.Source), "file") {
			builder.WriteString(secretRef)
			builder.WriteString("_FILE=")
			builder.WriteString(strings.TrimSpace(secret.Ref))
			builder.WriteString("\n")
		}
	}

	builder.WriteString("PAYWALL_WALLET_MODE=")
	builder.WriteString(strings.TrimSpace(paywallComponent.WalletMode))
	builder.WriteString("\n")
	builder.WriteString("PAYWALL_WALLET_RPC_URL=")
	builder.WriteString(strings.TrimSpace(paywallComponent.Settings["wallet_rpc_url"]))
	builder.WriteString("\n")
	builder.WriteString("STORE_PAYWALL_ENDPOINT=")
	builder.WriteString(strings.TrimSpace(storeComponent.Settings["paywall_endpoint"]))
	builder.WriteString("\n")
	return builder.String()
}

func renderFallbackSystemdUnit() string {
	var builder strings.Builder
	builder.WriteString("[Unit]\n")
	builder.WriteString("Description=Diagon fallback compose stack\n")
	builder.WriteString("After=network-online.target docker.service\n")
	builder.WriteString("Wants=network-online.target\n")
	builder.WriteString("Requires=docker.service\n\n")
	builder.WriteString("[Service]\n")
	builder.WriteString("Type=oneshot\n")
	builder.WriteString("RemainAfterExit=yes\n")
	builder.WriteString("WorkingDirectory=/opt/diagon/compose\n")
	builder.WriteString("ExecStart=/usr/bin/docker compose --env-file /etc/diagon/compose/diagon-compose.env -f /opt/diagon/compose/compose.yaml up -d\n")
	builder.WriteString("ExecStop=/usr/bin/docker compose --env-file /etc/diagon/compose/diagon-compose.env -f /opt/diagon/compose/compose.yaml down\n")
	builder.WriteString("TimeoutStartSec=180\n")
	builder.WriteString("TimeoutStopSec=120\n\n")
	builder.WriteString("[Install]\n")
	builder.WriteString("WantedBy=multi-user.target\n")
	return builder.String()
}

func renderFallbackManualGuide(bundle DebianComposeServiceBundle, bootstrap BootstrapProfile, contract ServiceContract, tunnelListeners []string) string {
	var builder strings.Builder
	builder.WriteString("# Diagon Debian Fallback Compose Bundle\n\n")
	builder.WriteString("This interim bundle is used when full Debian packaging cannot be shipped on schedule.\n\n")
	builder.WriteString("## Files\n\n")
	builder.WriteString("- compose: `")
	builder.WriteString(bundle.Compose.Path)
	builder.WriteString("`\n")
	builder.WriteString("- environment template: `")
	builder.WriteString(bundle.EnvironmentTemplate.Path)
	builder.WriteString("`\n")
	builder.WriteString("- systemd unit: `")
	builder.WriteString(bundle.SystemdUnit.Path)
	builder.WriteString("`\n")
	builder.WriteString("- manual guide: `")
	builder.WriteString(bundle.ManualInstallGuide.Path)
	builder.WriteString("`\n\n")
	builder.WriteString("## Startup Sequence\n\n")
	for _, serviceName := range bootstrap.StartupSequence {
		builder.WriteString("- ")
		builder.WriteString(strings.TrimSpace(serviceName))
		builder.WriteString("\n")
	}
	builder.WriteString("\n## Manual Install Steps\n\n")
	for idx, step := range bundle.ManualInstallSteps {
		builder.WriteString(fmt.Sprintf("%d. %s\n", idx+1, step))
	}
	builder.WriteString("\n## Verification Commands\n\n```bash\n")
	for _, check := range bundle.ValidationChecks {
		builder.WriteString(check.Command)
		builder.WriteString("\n")
	}
	builder.WriteString("```\n\n")
	builder.WriteString("## Expected Tunnel Listeners\n\n")
	for _, listener := range tunnelListeners {
		builder.WriteString("- `")
		builder.WriteString(listener)
		builder.WriteString("`\n")
	}
	builder.WriteString("\n## Service Contract\n\n")
	builder.WriteString("- services: ")
	builder.WriteString(fmt.Sprintf("%d", len(contract.Services)))
	builder.WriteString("\n")
	builder.WriteString("- i2pd tunnels: ")
	builder.WriteString(fmt.Sprintf("%d", len(contract.I2PDTunnels)))
	builder.WriteString("\n")
	if bundle.Environment != "" {
		builder.WriteString("- integration environment: `")
		builder.WriteString(bundle.Environment)
		builder.WriteString("`\n")
	}
	return builder.String()
}
