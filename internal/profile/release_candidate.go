package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

type IntegrationMatrix struct {
	SchemaVersion int                      `json:"schema_version"`
	Environments  []IntegrationEnvironment `json:"environments"`
}

type IntegrationEnvironment struct {
	Environment         string                      `json:"environment"`
	DebianVersion       string                      `json:"debian_version"`
	DebianCodename      string                      `json:"debian_codename"`
	GoVersion           string                      `json:"go_version,omitempty"`
	PackageDependencies []string                    `json:"package_dependencies"`
	Components          IntegrationMatrixComponents `json:"components"`
	ContractFixtures    IntegrationContractFixtures `json:"contract_fixtures"`
}

type IntegrationMatrixComponents struct {
	Diagon  IntegrationMatrixComponent `json:"diagon"`
	Store   IntegrationMatrixComponent `json:"store"`
	Paywall IntegrationMatrixComponent `json:"paywall"`
	I2PD    IntegrationMatrixComponent `json:"i2pd"`
}

type IntegrationMatrixComponent struct {
	Repo       string `json:"repo"`
	Version    string `json:"version"`
	BuildInput string `json:"build_input"`
}

type IntegrationContractFixtures struct {
	Primary          string   `json:"primary"`
	ServiceContracts []string `json:"service_contracts"`
}

type ReleaseCandidateSmokePlan struct {
	Name              string                            `json:"name"`
	WalletMode        string                            `json:"wallet_mode"`
	StartupSequence   []string                          `json:"startup_sequence"`
	ServiceEndpoints  []ReleaseCandidateServiceEndpoint `json:"service_endpoints"`
	TunnelEndpoints   []ReleaseCandidateTunnelEndpoint  `json:"tunnel_endpoints"`
	HealthChecks      []ReleaseCandidateSmokeCheck      `json:"health_checks"`
	MarketplaceAccess ReleaseCandidateSmokeCheck        `json:"marketplace_access"`
	PaywallValidation ReleaseCandidateSmokeCheck        `json:"paywall_validation"`
	GracefulRestart   ReleaseCandidateRestartPlan       `json:"graceful_restart"`
	SuccessCriteria   []string                          `json:"success_criteria"`
}

type ReleaseCandidateServiceEndpoint struct {
	Service   string `json:"service"`
	Listen    string `json:"listen"`
	BaseURL   string `json:"base_url"`
	HealthURL string `json:"health_url"`
}

type ReleaseCandidateTunnelEndpoint struct {
	Name          string `json:"name"`
	TargetService string `json:"target_service"`
	Listen        string `json:"listen"`
	BaseURL       string `json:"base_url"`
	Target        string `json:"target"`
	Type          string `json:"type"`
}

type ReleaseCandidateSmokeCheck struct {
	Service            string   `json:"service"`
	Method             string   `json:"method"`
	URL                string   `json:"url"`
	ExpectedStatus     int      `json:"expected_status"`
	ExpectedAssertions []string `json:"expected_assertions,omitempty"`
	ViaTunnel          string   `json:"via_tunnel,omitempty"`
}

type ReleaseCandidateRestartPlan struct {
	StopUnits         []string                     `json:"stop_units"`
	StartUnits        []string                     `json:"start_units"`
	PostRestartChecks []ReleaseCandidateSmokeCheck `json:"post_restart_checks"`
}

type ReleaseCandidateBaseline struct {
	Environment         string                                `json:"environment"`
	DebianVersion       string                                `json:"debian_version"`
	DebianCodename      string                                `json:"debian_codename"`
	GoVersion           string                                `json:"go_version,omitempty"`
	TagName             string                                `json:"tag_name"`
	PackageDependencies []string                              `json:"package_dependencies"`
	ContractFixtures    []string                              `json:"contract_fixtures"`
	Components          map[string]IntegrationMatrixComponent `json:"components"`
	QualityGates        []string                              `json:"quality_gates"`
	SuccessCriteria     []string                              `json:"success_criteria"`
}

func LoadIntegrationMatrix(path string) (IntegrationMatrix, error) {
	if strings.TrimSpace(path) == "" {
		return IntegrationMatrix{}, fmt.Errorf("integration matrix path cannot be empty")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return IntegrationMatrix{}, fmt.Errorf("read integration matrix file %s: %w", path, err)
	}

	var matrix IntegrationMatrix
	if err := json.Unmarshal(raw, &matrix); err != nil {
		return IntegrationMatrix{}, fmt.Errorf("parse integration matrix file %s: %w", path, err)
	}
	if len(matrix.Environments) == 0 {
		return IntegrationMatrix{}, fmt.Errorf("integration matrix file %s must include at least one environment", path)
	}

	return matrix, nil
}

func (matrix IntegrationMatrix) EnvironmentByName(name string) (IntegrationEnvironment, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return IntegrationEnvironment{}, fmt.Errorf("integration environment cannot be empty")
	}

	for _, environment := range matrix.Environments {
		if strings.TrimSpace(environment.Environment) == trimmedName {
			return environment, nil
		}
	}

	return IntegrationEnvironment{}, fmt.Errorf("integration environment %q not found in matrix", trimmedName)
}

func BuildReleaseCandidateSmokePlan(bootstrap BootstrapProfile, contract ServiceContract) (ReleaseCandidateSmokePlan, error) {
	contractResult := ValidateServiceContractDefinition(contract)
	if contractResult.HasErrors() {
		return ReleaseCandidateSmokePlan{}, fmt.Errorf("invalid service contract: %s", strings.Join(contractResult.Errors, "; "))
	}

	bootstrapResult := ValidateBootstrapProfileDefinition(bootstrap, &contract)
	if bootstrapResult.HasErrors() {
		return ReleaseCandidateSmokePlan{}, fmt.Errorf("invalid bootstrap profile: %s", strings.Join(bootstrapResult.Errors, "; "))
	}

	components := make(map[string]BootstrapComponent, len(bootstrap.Components))
	for _, component := range bootstrap.Components {
		components[strings.TrimSpace(component.Name)] = component
	}

	paywallComponent, hasPaywall := components[requiredServicePaywall]
	storeComponent, hasStore := components[requiredServiceStore]
	if !hasPaywall || !hasStore {
		return ReleaseCandidateSmokePlan{}, fmt.Errorf("bootstrap profile must define %q and %q components", requiredServicePaywall, requiredServiceStore)
	}

	checkoutPath, err := normalizedRequestPath(storeComponent.Settings["smoke_checkout_path"], "store settings.smoke_checkout_path")
	if err != nil {
		return ReleaseCandidateSmokePlan{}, err
	}
	paymentPath, err := normalizedRequestPath(paywallComponent.Settings["smoke_payment_path"], "paywall settings.smoke_payment_path")
	if err != nil {
		return ReleaseCandidateSmokePlan{}, err
	}

	serviceEndpoints := make([]ReleaseCandidateServiceEndpoint, 0, len(contract.Services))
	healthChecks := make([]ReleaseCandidateSmokeCheck, 0, len(contract.Services))
	for _, service := range servicesInStartupOrder(contract.Services) {
		baseURL := asHTTPURL(service.Listen)
		serviceEndpoints = append(serviceEndpoints, ReleaseCandidateServiceEndpoint{
			Service:   strings.TrimSpace(service.Name),
			Listen:    strings.TrimSpace(service.Listen),
			BaseURL:   baseURL,
			HealthURL: strings.TrimSpace(service.HealthURL),
		})
		healthChecks = append(healthChecks, ReleaseCandidateSmokeCheck{
			Service:        strings.TrimSpace(service.Name),
			Method:         "GET",
			URL:            strings.TrimSpace(service.HealthURL),
			ExpectedStatus: 200,
		})
	}

	tunnelEndpoints := make([]ReleaseCandidateTunnelEndpoint, 0, len(contract.I2PDTunnels))
	storeTunnelURL := ""
	for _, tunnel := range contract.I2PDTunnels {
		baseURL := asHTTPURL(tunnel.Listen)
		tunnelEndpoints = append(tunnelEndpoints, ReleaseCandidateTunnelEndpoint{
			Name:          strings.TrimSpace(tunnel.Name),
			TargetService: strings.TrimSpace(tunnel.TargetService),
			Listen:        strings.TrimSpace(tunnel.Listen),
			BaseURL:       baseURL,
			Target:        strings.TrimSpace(tunnel.Target),
			Type:          strings.TrimSpace(tunnel.Type),
		})
		if storeTunnelURL == "" && strings.TrimSpace(tunnel.TargetService) == requiredServiceStore {
			storeTunnelURL = baseURL
		}
	}
	if storeTunnelURL == "" {
		return ReleaseCandidateSmokePlan{}, fmt.Errorf("service contract must define at least one store tunnel for release candidate smoke flow")
	}

	startUnits := make([]string, 0, len(contract.Services))
	for _, serviceName := range contractServiceNamesInStartupOrder(contract) {
		startUnits = append(startUnits, debianUnitName(serviceName))
	}

	return ReleaseCandidateSmokePlan{
		Name:             strings.TrimSpace(bootstrap.Name),
		WalletMode:       strings.TrimSpace(paywallComponent.WalletMode),
		StartupSequence:  append([]string{}, bootstrap.StartupSequence...),
		ServiceEndpoints: serviceEndpoints,
		TunnelEndpoints:  tunnelEndpoints,
		HealthChecks:     healthChecks,
		MarketplaceAccess: ReleaseCandidateSmokeCheck{
			Service:        requiredServiceStore,
			Method:         "POST",
			URL:            storeTunnelURL + checkoutPath,
			ExpectedStatus: 200,
			ExpectedAssertions: []string{
				`json.checkout == "ok"`,
				`json.paywall.settled == true`,
			},
			ViaTunnel: storeTunnelURL,
		},
		PaywallValidation: ReleaseCandidateSmokeCheck{
			Service:        requiredServicePaywall,
			Method:         "POST",
			URL:            asHTTPURL(paywallComponent.Listen) + paymentPath,
			ExpectedStatus: 200,
			ExpectedAssertions: []string{
				`json.settled == true`,
			},
		},
		GracefulRestart: ReleaseCandidateRestartPlan{
			StopUnits:         reverseCopy(startUnits),
			StartUnits:        append([]string{}, startUnits...),
			PostRestartChecks: append([]ReleaseCandidateSmokeCheck{}, healthChecks...),
		},
		SuccessCriteria: []string{
			"All services report 2xx health status in startup order.",
			"Marketplace access succeeds through the Store i2pd tunnel endpoint.",
			"Paywall validation returns a settled response in stubbed wallet mode.",
			"After a graceful restart, all health checks and the marketplace access path succeed again.",
		},
	}, nil
}

func WriteReleaseCandidateSmokePlan(path string, plan ReleaseCandidateSmokePlan) error {
	return writeJSONFile(path, plan, "release candidate smoke plan")
}

func BuildOperatorRunbook(bootstrap BootstrapProfile, contract ServiceContract, environment *IntegrationEnvironment) (string, error) {
	smokePlan, err := BuildReleaseCandidateSmokePlan(bootstrap, contract)
	if err != nil {
		return "", err
	}

	debianPlan, err := BuildDebianPackagePlan(bootstrap, contract)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("# Diagon Operator Runbook\n\n")
	builder.WriteString("## Scope\n\n")
	builder.WriteString("This runbook covers local single-host operation of Diagon, Store, Paywall, and i2pd on Debian.\n\n")
	if environment != nil {
		builder.WriteString("- Environment: ")
		builder.WriteString(strings.TrimSpace(environment.Environment))
		builder.WriteString("\n")
		builder.WriteString("- Debian: ")
		builder.WriteString(strings.TrimSpace(environment.DebianVersion))
		builder.WriteString(" (")
		builder.WriteString(strings.TrimSpace(environment.DebianCodename))
		builder.WriteString(")\n")
		builder.WriteString("- Component versions: diagon=")
		builder.WriteString(strings.TrimSpace(environment.Components.Diagon.Version))
		builder.WriteString(", store=")
		builder.WriteString(strings.TrimSpace(environment.Components.Store.Version))
		builder.WriteString(", paywall=")
		builder.WriteString(strings.TrimSpace(environment.Components.Paywall.Version))
		builder.WriteString(", i2pd=")
		builder.WriteString(strings.TrimSpace(environment.Components.I2PD.Version))
		builder.WriteString("\n\n")
	}

	builder.WriteString("## Config And Secrets\n\n")
	for _, component := range bootstrap.Components {
		builder.WriteString("- ")
		builder.WriteString(strings.TrimSpace(component.Name))
		builder.WriteString(" config: ")
		builder.WriteString(strings.TrimSpace(component.ConfigPath))
		builder.WriteString("\n")
	}
	for _, secret := range bootstrap.Secrets {
		builder.WriteString("- Secret ")
		builder.WriteString(strings.TrimSpace(secret.Name))
		builder.WriteString(": ")
		builder.WriteString(strings.TrimSpace(secret.Source))
		builder.WriteString(" -> ")
		builder.WriteString(strings.TrimSpace(secret.Ref))
		if secret.Required {
			builder.WriteString(" (required)")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("\n")

	builder.WriteString("## Start\n\n```bash\n")
	for _, unit := range smokePlan.GracefulRestart.StartUnits {
		builder.WriteString("sudo systemctl start ")
		builder.WriteString(unit)
		builder.WriteString("\n")
	}
	builder.WriteString("```\n\n")

	builder.WriteString("## Stop\n\n```bash\n")
	for _, unit := range smokePlan.GracefulRestart.StopUnits {
		builder.WriteString("sudo systemctl stop ")
		builder.WriteString(unit)
		builder.WriteString("\n")
	}
	builder.WriteString("```\n\n")

	builder.WriteString("## Status\n\n```bash\n")
	for _, unit := range smokePlan.GracefulRestart.StartUnits {
		builder.WriteString("systemctl status ")
		builder.WriteString(unit)
		builder.WriteString(" --no-pager\n")
	}
	for _, check := range smokePlan.HealthChecks {
		builder.WriteString("curl -fsS ")
		builder.WriteString(check.URL)
		builder.WriteString("\n")
	}
	builder.WriteString("```\n\n")

	builder.WriteString("## Logs\n\n```bash\n")
	for _, unit := range smokePlan.GracefulRestart.StartUnits {
		builder.WriteString("journalctl -u ")
		builder.WriteString(unit)
		builder.WriteString(" -n 200 --no-pager\n")
	}
	builder.WriteString("```\n\n")

	builder.WriteString("## Smoke Validation\n\n```bash\n")
	builder.WriteString("curl -fsS -X ")
	builder.WriteString(smokePlan.PaywallValidation.Method)
	builder.WriteString(" ")
	builder.WriteString(smokePlan.PaywallValidation.URL)
	builder.WriteString("\n")
	builder.WriteString("curl -fsS -X ")
	builder.WriteString(smokePlan.MarketplaceAccess.Method)
	builder.WriteString(" ")
	builder.WriteString(smokePlan.MarketplaceAccess.URL)
	builder.WriteString("\n```\n\n")

	builder.WriteString("## Recovery\n\n")
	builder.WriteString("1. If a single service is unhealthy, inspect its unit status and logs, then restart that unit.\n")
	builder.WriteString("2. If dependency order is suspect, stop services in reverse order and start them again in startup order.\n")
	builder.WriteString("3. Re-run the health checks and smoke validation commands before returning the host to service.\n")
	builder.WriteString("4. Preserve operator-managed state under /etc/diagon, /var/lib/diagon, and /var/log/diagon during rollback.\n\n")

	builder.WriteString("## Package-Owned Units\n\n")
	for _, unit := range debianPlan.ServiceUnits {
		builder.WriteString("- ")
		builder.WriteString(unit.Name)
		builder.WriteString(": ")
		builder.WriteString(unit.Description)
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

func WriteOperatorRunbook(path, runbook string) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("operator runbook output path cannot be empty")
	}

	raw := []byte(runbook)
	if !strings.HasSuffix(runbook, "\n") {
		raw = append(raw, '\n')
	}

	if trimmedPath == "-" {
		if _, err := os.Stdout.Write(raw); err != nil {
			return fmt.Errorf("write operator runbook to stdout: %w", err)
		}
		return nil
	}

	if err := os.WriteFile(trimmedPath, raw, 0o644); err != nil {
		return fmt.Errorf("write operator runbook to %s: %w", trimmedPath, err)
	}
	return nil
}

func BuildWalletValidationChecklist(bootstrap BootstrapProfile, contract ServiceContract, environment *IntegrationEnvironment) (string, error) {
	contractResult := ValidateServiceContractDefinition(contract)
	if contractResult.HasErrors() {
		return "", fmt.Errorf("invalid service contract: %s", strings.Join(contractResult.Errors, "; "))
	}

	bootstrapResult := ValidateBootstrapProfileDefinition(bootstrap, &contract)
	if bootstrapResult.HasErrors() {
		return "", fmt.Errorf("invalid bootstrap profile: %s", strings.Join(bootstrapResult.Errors, "; "))
	}

	componentByName := make(map[string]BootstrapComponent, len(bootstrap.Components))
	for _, component := range bootstrap.Components {
		componentByName[strings.TrimSpace(component.Name)] = component
	}

	paywallComponent, exists := componentByName[requiredServicePaywall]
	if !exists {
		return "", fmt.Errorf("bootstrap profile must define %q component", requiredServicePaywall)
	}

	walletRPCURL := strings.TrimSpace(paywallComponent.Settings["wallet_rpc_url"])
	if walletRPCURL == "" {
		return "", fmt.Errorf("bootstrap component %q must define settings.wallet_rpc_url", requiredServicePaywall)
	}

	paymentPath := strings.TrimSpace(paywallComponent.Settings["smoke_payment_path"])
	if paymentPath == "" {
		paymentPath = "/pay"
	}
	paymentPath, err := normalizedRequestPath(paymentPath, "paywall settings.smoke_payment_path")
	if err != nil {
		return "", err
	}

	secretByName := make(map[string]BootstrapSecret, len(bootstrap.Secrets))
	for _, secret := range bootstrap.Secrets {
		secretByName[strings.TrimSpace(secret.Name)] = secret
	}

	secretRefs := uniqueSortedStrings(paywallComponent.SecretRefs)

	var builder strings.Builder
	builder.WriteString("# Diagon Production Wallet Validation Checklist\n\n")
	builder.WriteString("This checklist validates Monero wallet RPC readiness for production deployments after CI has passed in stubbed mode.\n\n")
	if environment != nil {
		builder.WriteString("- Environment: ")
		builder.WriteString(strings.TrimSpace(environment.Environment))
		builder.WriteString("\n")
		builder.WriteString("- Debian: ")
		builder.WriteString(strings.TrimSpace(environment.DebianVersion))
		builder.WriteString(" (")
		builder.WriteString(strings.TrimSpace(environment.DebianCodename))
		builder.WriteString(")\n")
		builder.WriteString("- Component versions: diagon=")
		builder.WriteString(strings.TrimSpace(environment.Components.Diagon.Version))
		builder.WriteString(", store=")
		builder.WriteString(strings.TrimSpace(environment.Components.Store.Version))
		builder.WriteString(", paywall=")
		builder.WriteString(strings.TrimSpace(environment.Components.Paywall.Version))
		builder.WriteString(", i2pd=")
		builder.WriteString(strings.TrimSpace(environment.Components.I2PD.Version))
		builder.WriteString("\n\n")
	}

	builder.WriteString("## CI Baseline\n\n")
	builder.WriteString("- [ ] Confirm CI Stage 6 smoke output reports `wallet_mode: stubbed`.\n")
	builder.WriteString("- [ ] Confirm production deployment replaces stubbed wallet mode with a real wallet RPC target.\n\n")

	builder.WriteString("## Secrets And Endpoint Preflight\n\n")
	builder.WriteString("- Wallet RPC URL: `")
	builder.WriteString(walletRPCURL)
	builder.WriteString("`\n")
	builder.WriteString("- Paywall health endpoint: `")
	builder.WriteString(strings.TrimSpace(paywallComponent.HealthURL))
	builder.WriteString("`\n")
	builder.WriteString("- Paywall settlement endpoint path: `")
	builder.WriteString(paymentPath)
	builder.WriteString("`\n")
	if len(secretRefs) == 0 {
		builder.WriteString("- [ ] No paywall secret references were declared; verify production credentials are injected externally.\n")
	} else {
		for _, secretRef := range secretRefs {
			secret := secretByName[secretRef]
			builder.WriteString("- [ ] Verify secret `")
			builder.WriteString(secretRef)
			builder.WriteString("` is present via ")
			builder.WriteString(strings.TrimSpace(secret.Source))
			builder.WriteString(" -> `")
			builder.WriteString(strings.TrimSpace(secret.Ref))
			builder.WriteString("`\n")
		}
	}
	builder.WriteString("\n")

	builder.WriteString("## Wallet RPC Validation Commands\n\n```bash\n")
	builder.WriteString("curl -fsS -X POST ")
	builder.WriteString(walletRPCURL)
	builder.WriteString(" \\\n")
	builder.WriteString("  -H 'Content-Type: application/json' \\\n")
	builder.WriteString("  --data '{\"jsonrpc\":\"2.0\",\"id\":\"diagon-wallet-check\",\"method\":\"get_version\"}'\n")
	builder.WriteString("\n")
	builder.WriteString("curl -fsS ")
	builder.WriteString(strings.TrimSpace(paywallComponent.HealthURL))
	builder.WriteString("\n")
	builder.WriteString("\n")
	builder.WriteString("curl -fsS -X POST http://")
	builder.WriteString(strings.TrimSpace(paywallComponent.Listen))
	builder.WriteString(paymentPath)
	builder.WriteString(" \\\n")
	builder.WriteString("  -H 'Content-Type: application/json' \\\n")
	builder.WriteString("  --data '{\"amount\":1,\"currency\":\"XMR\"}'\n")
	builder.WriteString("```\n\n")

	builder.WriteString("## Success Criteria\n\n")
	builder.WriteString("- [ ] Wallet RPC responds to `get_version` without timeout or auth errors.\n")
	builder.WriteString("- [ ] Paywall health endpoint returns `2xx` after production wallet settings are applied.\n")
	builder.WriteString("- [ ] Paywall settlement path accepts a test request and returns a successful response.\n")
	builder.WriteString("- [ ] Operator captures command output and rollback steps in deployment records.\n")

	return builder.String(), nil
}

func WriteWalletValidationChecklist(path, checklist string) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("wallet validation checklist output path cannot be empty")
	}

	raw := []byte(checklist)
	if !strings.HasSuffix(checklist, "\n") {
		raw = append(raw, '\n')
	}

	if trimmedPath == "-" {
		if _, err := os.Stdout.Write(raw); err != nil {
			return fmt.Errorf("write wallet validation checklist to stdout: %w", err)
		}
		return nil
	}

	if err := os.WriteFile(trimmedPath, raw, 0o644); err != nil {
		return fmt.Errorf("write wallet validation checklist to %s: %w", trimmedPath, err)
	}
	return nil
}

func BuildReleaseCandidateBaseline(matrix IntegrationMatrix, environmentName string) (ReleaseCandidateBaseline, error) {
	environment, err := matrix.EnvironmentByName(environmentName)
	if err != nil {
		return ReleaseCandidateBaseline{}, err
	}

	if strings.TrimSpace(environment.ContractFixtures.Primary) == "" {
		return ReleaseCandidateBaseline{}, fmt.Errorf("integration environment %q must define contract_fixtures.primary", environment.Environment)
	}

	fixtures := append([]string{strings.TrimSpace(environment.ContractFixtures.Primary)}, environment.ContractFixtures.ServiceContracts...)
	fixtures = uniqueSortedStrings(fixtures)
	tagVersion, err := deriveReleaseCandidateTagVersion(fixtures)
	if err != nil {
		return ReleaseCandidateBaseline{}, err
	}

	components := map[string]IntegrationMatrixComponent{
		"diagon":  environment.Components.Diagon,
		"store":   environment.Components.Store,
		"paywall": environment.Components.Paywall,
		"i2pd":    environment.Components.I2PD,
	}
	for name, component := range components {
		if strings.TrimSpace(component.Repo) == "" || strings.TrimSpace(component.Version) == "" || strings.TrimSpace(component.BuildInput) == "" {
			return ReleaseCandidateBaseline{}, fmt.Errorf("integration environment %q component %q must define repo, version, and build_input", environment.Environment, name)
		}
	}

	return ReleaseCandidateBaseline{
		Environment:         strings.TrimSpace(environment.Environment),
		DebianVersion:       strings.TrimSpace(environment.DebianVersion),
		DebianCodename:      strings.TrimSpace(environment.DebianCodename),
		GoVersion:           strings.TrimSpace(environment.GoVersion),
		TagName:             fmt.Sprintf("integration-%s-%s", strings.TrimSpace(environment.Environment), tagVersion),
		PackageDependencies: append([]string{}, environment.PackageDependencies...),
		ContractFixtures:    fixtures,
		Components:          components,
		QualityGates: []string{
			"merge blocked unless Stages 1 through 7 pass",
			"release blocked unless Stage 8 passes",
		},
		SuccessCriteria: []string{
			"Pinned component versions match the integration matrix.",
			"Service-contract fixtures are frozen under the release candidate baseline tag.",
			"The release bundle includes checksums, the version manifest, and the operator runbook.",
		},
	}, nil
}

func WriteReleaseCandidateBaseline(path string, baseline ReleaseCandidateBaseline) error {
	return writeJSONFile(path, baseline, "release candidate baseline")
}

func normalizedRequestPath(raw, fieldName string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%s cannot be empty", fieldName)
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return trimmed, nil
}

func deriveReleaseCandidateTagVersion(fixtures []string) (string, error) {
	versionPattern := regexp.MustCompile(`v\d{4}\.\d{2}\.\d{2}`)
	versions := make([]string, 0, len(fixtures))
	for _, fixture := range fixtures {
		if match := versionPattern.FindString(fixture); match != "" {
			versions = append(versions, match)
		}
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no versioned service-contract fixture found for release candidate baseline")
	}
	sort.Strings(versions)
	return versions[len(versions)-1], nil
}

func writeJSONFile(path string, payload any, fileLabel string) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("%s output path cannot be empty", fileLabel)
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", fileLabel, err)
	}
	raw = append(raw, '\n')

	if trimmedPath == "-" {
		if _, err := os.Stdout.Write(raw); err != nil {
			return fmt.Errorf("write %s to stdout: %w", fileLabel, err)
		}
		return nil
	}

	if err := os.WriteFile(trimmedPath, raw, 0o644); err != nil {
		return fmt.Errorf("write %s to %s: %w", fileLabel, trimmedPath, err)
	}
	return nil
}
