package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	debianPackageName = "diagon"
	debianConfigRoot  = "/etc/diagon"
	debianLibexecRoot = "/usr/libexec/diagon"
	debianLogRoot     = "/var/log/diagon"
	debianStateRoot   = "/var/lib/diagon"
	debianRuntimeRoot = "/run/diagon"
)

type DebianPackagePlan struct {
	PackageName  string                `json:"package_name"`
	Layout       DebianPackageLayout   `json:"layout"`
	ServiceUnits []DebianServiceUnit   `json:"service_units"`
	PostInstall  DebianPostInstallPlan `json:"post_install"`
	Uninstall    DebianUninstallPlan   `json:"uninstall"`
}

type DebianPackageLayout struct {
	Binaries           []DebianPathEntry `json:"binaries"`
	ConfigDirectories  []string          `json:"config_directories"`
	ConfigFiles        []string          `json:"config_files"`
	LogDirectories     []string          `json:"log_directories"`
	StateDirectories   []string          `json:"state_directories"`
	RuntimeDirectories []string          `json:"runtime_directories"`
}

type DebianPathEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Source string `json:"source"`
}

type DebianServiceUnit struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	After            []string `json:"after,omitempty"`
	Requires         []string `json:"requires,omitempty"`
	Wants            []string `json:"wants,omitempty"`
	WantedBy         []string `json:"wanted_by,omitempty"`
	Restart          string   `json:"restart"`
	ExecStart        []string `json:"exec_start"`
	WorkingDirectory string   `json:"working_directory"`
	ReadWritePaths   []string `json:"read_write_paths,omitempty"`
	UnitFile         string   `json:"unit_file"`
}

type DebianPostInstallPlan struct {
	EnableUnits        []string                `json:"enable_units"`
	StartUnits         []string                `json:"start_units"`
	RequiredFiles      []string                `json:"required_files"`
	HealthChecks       []DebianHealthCheck     `json:"health_checks"`
	SecretRequirements []BootstrapSecret       `json:"secret_requirements,omitempty"`
	ValidationChecks   []DebianValidationCheck `json:"validation_checks"`
}

type DebianHealthCheck struct {
	Service string `json:"service"`
	URL     string `json:"url"`
}

type DebianValidationCheck struct {
	Type   string `json:"type"`
	Target string `json:"target"`
	Expect string `json:"expect"`
}

type DebianUninstallPlan struct {
	StopUnits     []string `json:"stop_units"`
	DisableUnits  []string `json:"disable_units"`
	PreservePaths []string `json:"preserve_paths"`
	RemovePaths   []string `json:"remove_paths"`
	Notes         []string `json:"notes"`
}

func BuildDebianPackagePlan(bootstrap BootstrapProfile, contract ServiceContract) (DebianPackagePlan, error) {
	contractResult := ValidateServiceContractDefinition(contract)
	if contractResult.HasErrors() {
		return DebianPackagePlan{}, fmt.Errorf("invalid service contract: %s", strings.Join(contractResult.Errors, "; "))
	}

	bootstrapResult := ValidateBootstrapProfileDefinition(bootstrap, &contract)
	if bootstrapResult.HasErrors() {
		return DebianPackagePlan{}, fmt.Errorf("invalid bootstrap profile: %s", strings.Join(bootstrapResult.Errors, "; "))
	}

	components := make(map[string]BootstrapComponent, len(bootstrap.Components))
	for _, component := range bootstrap.Components {
		components[strings.TrimSpace(component.Name)] = component
	}

	i2pdComponent, hasI2PD := components[requiredServiceI2PD]
	paywallComponent, hasPaywall := components[requiredServicePaywall]
	storeComponent, hasStore := components[requiredServiceStore]
	if !hasI2PD || !hasPaywall || !hasStore {
		return DebianPackagePlan{}, fmt.Errorf("bootstrap profile must define %q, %q, and %q components", requiredServiceI2PD, requiredServicePaywall, requiredServiceStore)
	}

	configDirs := uniqueSortedStrings([]string{
		debianConfigRoot,
		filepath.Dir(strings.TrimSpace(i2pdComponent.ConfigPath)),
		filepath.Dir(strings.TrimSpace(i2pdComponent.Settings["tunnel_config_path"])),
		filepath.Dir(strings.TrimSpace(paywallComponent.ConfigPath)),
		filepath.Dir(strings.TrimSpace(storeComponent.ConfigPath)),
	})

	configFiles := uniqueSortedStrings([]string{
		strings.TrimSpace(i2pdComponent.ConfigPath),
		strings.TrimSpace(i2pdComponent.Settings["tunnel_config_path"]),
		strings.TrimSpace(paywallComponent.ConfigPath),
		strings.TrimSpace(storeComponent.ConfigPath),
	})

	serviceNames := contractServiceNamesInStartupOrder(contract)
	logDirs := make([]string, 0, len(serviceNames))
	stateDirs := make([]string, 0, len(serviceNames))
	runtimeDirs := make([]string, 0, len(serviceNames))
	for _, serviceName := range serviceNames {
		logDirs = append(logDirs, filepath.Join(debianLogRoot, serviceName))
		stateDirs = append(stateDirs, filepath.Join(debianStateRoot, serviceName))
		runtimeDirs = append(runtimeDirs, filepath.Join(debianRuntimeRoot, serviceName))
	}

	serviceUnits := make([]DebianServiceUnit, 0, len(contract.Services))
	for _, service := range servicesInStartupOrder(contract.Services) {
		component := components[strings.TrimSpace(service.Name)]
		serviceUnits = append(serviceUnits, buildDebianServiceUnit(service, component))
	}

	enabledUnits := make([]string, 0, len(serviceUnits))
	startUnits := make([]string, 0, len(serviceUnits))
	healthChecks := make([]DebianHealthCheck, 0, len(contract.Services))
	validationChecks := make([]DebianValidationCheck, 0, len(serviceUnits)+len(configFiles)+len(contract.Services))
	for _, unit := range serviceUnits {
		enabledUnits = append(enabledUnits, unit.Name)
		startUnits = append(startUnits, unit.Name)
		validationChecks = append(validationChecks,
			DebianValidationCheck{Type: "systemd-enabled", Target: unit.Name, Expect: "enabled"},
			DebianValidationCheck{Type: "systemd-active", Target: unit.Name, Expect: "active"},
		)
	}
	for _, configFile := range configFiles {
		validationChecks = append(validationChecks, DebianValidationCheck{Type: "file-exists", Target: configFile, Expect: "present"})
	}
	for _, service := range servicesInStartupOrder(contract.Services) {
		healthChecks = append(healthChecks, DebianHealthCheck{Service: service.Name, URL: strings.TrimSpace(service.HealthURL)})
		validationChecks = append(validationChecks, DebianValidationCheck{Type: "http-2xx", Target: strings.TrimSpace(service.HealthURL), Expect: "reachable"})
	}

	plan := DebianPackagePlan{
		PackageName: debianPackageName,
		Layout: DebianPackageLayout{
			Binaries: []DebianPathEntry{
				{Name: "diagonctl", Path: "/usr/bin/diagonctl", Source: "diagon-package"},
				{Name: "store", Path: filepath.Join(debianLibexecRoot, "store"), Source: "diagon-package"},
				{Name: "paywall", Path: filepath.Join(debianLibexecRoot, "paywall"), Source: "diagon-package"},
				{Name: "i2pd", Path: "/usr/sbin/i2pd", Source: "debian-dependency"},
			},
			ConfigDirectories:  configDirs,
			ConfigFiles:        configFiles,
			LogDirectories:     uniqueSortedStrings(logDirs),
			StateDirectories:   uniqueSortedStrings(stateDirs),
			RuntimeDirectories: uniqueSortedStrings(runtimeDirs),
		},
		ServiceUnits: serviceUnits,
		PostInstall: DebianPostInstallPlan{
			EnableUnits:        enabledUnits,
			StartUnits:         startUnits,
			RequiredFiles:      configFiles,
			HealthChecks:       healthChecks,
			SecretRequirements: append([]BootstrapSecret{}, bootstrap.Secrets...),
			ValidationChecks:   validationChecks,
		},
		Uninstall: DebianUninstallPlan{
			StopUnits:    reverseCopy(startUnits),
			DisableUnits: reverseCopy(enabledUnits),
			PreservePaths: uniqueSortedStrings([]string{
				debianConfigRoot,
				debianLogRoot,
				debianStateRoot,
			}),
			RemovePaths: []string{debianRuntimeRoot},
			Notes: []string{
				"Stop services in reverse dependency order before removing package-owned files.",
				"Preserve operator-managed configuration and application state under /etc/diagon and /var/lib/diagon.",
				"Retain logs under /var/log/diagon for rollback analysis; only runtime sockets and pid files under /run/diagon are removed.",
			},
		},
	}

	return plan, nil
}

func WriteDebianPackagePlan(path string, plan DebianPackagePlan) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("debian package output path cannot be empty")
	}

	raw, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal debian package plan: %w", err)
	}
	raw = append(raw, '\n')

	if trimmedPath == "-" {
		if _, err := os.Stdout.Write(raw); err != nil {
			return fmt.Errorf("write debian package plan to stdout: %w", err)
		}
		return nil
	}

	if err := os.WriteFile(trimmedPath, raw, 0o644); err != nil {
		return fmt.Errorf("write debian package plan to %s: %w", trimmedPath, err)
	}
	return nil
}

func buildDebianServiceUnit(service ServiceDefinition, component BootstrapComponent) DebianServiceUnit {
	serviceName := strings.TrimSpace(service.Name)
	unitName := debianUnitName(serviceName)
	after := make([]string, 0, len(service.DependsOn)+1)
	requires := make([]string, 0, len(service.DependsOn))
	wants := []string{"network-online.target"}
	if serviceName == requiredServiceI2PD {
		after = append(after, "network-online.target")
	} else {
		after = append(after, "network-online.target")
		for _, dependency := range service.DependsOn {
			trimmedDependency := strings.TrimSpace(dependency)
			if trimmedDependency == "" {
				continue
			}
			dependencyUnit := debianUnitName(trimmedDependency)
			after = append(after, dependencyUnit)
			requires = append(requires, dependencyUnit)
		}
	}

	configDir := filepath.Dir(strings.TrimSpace(component.ConfigPath))
	readWritePaths := uniqueSortedStrings([]string{
		configDir,
		filepath.Join(debianLogRoot, serviceName),
		filepath.Join(debianRuntimeRoot, serviceName),
		filepath.Join(debianStateRoot, serviceName),
	})

	execStart := debianExecStart(serviceName, component)
	unit := DebianServiceUnit{
		Name:             unitName,
		Description:      "Diagon " + serviceName + " service",
		After:            uniqueSortedStrings(after),
		Requires:         uniqueSortedStrings(requires),
		Wants:            wants,
		WantedBy:         []string{"multi-user.target"},
		Restart:          "on-failure",
		ExecStart:        execStart,
		WorkingDirectory: filepath.Join(debianStateRoot, serviceName),
		ReadWritePaths:   readWritePaths,
	}
	unit.UnitFile = renderDebianServiceUnitFile(unit)
	return unit
}

func debianExecStart(serviceName string, component BootstrapComponent) []string {
	switch serviceName {
	case requiredServiceI2PD:
		return []string{"/usr/sbin/i2pd", "--conf", strings.TrimSpace(component.ConfigPath), "--tunconf", strings.TrimSpace(component.Settings["tunnel_config_path"])}
	case requiredServicePaywall:
		return []string{filepath.Join(debianLibexecRoot, "paywall"), "--config", strings.TrimSpace(component.ConfigPath)}
	case requiredServiceStore:
		return []string{filepath.Join(debianLibexecRoot, "store"), "--config", strings.TrimSpace(component.ConfigPath)}
	default:
		return []string{filepath.Join(debianLibexecRoot, serviceName), "--config", strings.TrimSpace(component.ConfigPath)}
	}
}

func renderDebianServiceUnitFile(unit DebianServiceUnit) string {
	var builder strings.Builder
	builder.WriteString("[Unit]\n")
	builder.WriteString("Description=")
	builder.WriteString(unit.Description)
	builder.WriteString("\n")
	if len(unit.After) > 0 {
		builder.WriteString("After=")
		builder.WriteString(strings.Join(unit.After, " "))
		builder.WriteString("\n")
	}
	if len(unit.Requires) > 0 {
		builder.WriteString("Requires=")
		builder.WriteString(strings.Join(unit.Requires, " "))
		builder.WriteString("\n")
	}
	if len(unit.Wants) > 0 {
		builder.WriteString("Wants=")
		builder.WriteString(strings.Join(unit.Wants, " "))
		builder.WriteString("\n")
	}
	builder.WriteString("\n[Service]\n")
	builder.WriteString("Type=simple\n")
	builder.WriteString("WorkingDirectory=")
	builder.WriteString(unit.WorkingDirectory)
	builder.WriteString("\n")
	builder.WriteString("ExecStart=")
	builder.WriteString(strings.Join(unit.ExecStart, " "))
	builder.WriteString("\n")
	builder.WriteString("Restart=")
	builder.WriteString(unit.Restart)
	builder.WriteString("\nRestartSec=5s\n")
	if len(unit.ReadWritePaths) > 0 {
		builder.WriteString("ReadWritePaths=")
		builder.WriteString(strings.Join(unit.ReadWritePaths, " "))
		builder.WriteString("\n")
	}
	builder.WriteString("\n[Install]\n")
	builder.WriteString("WantedBy=")
	builder.WriteString(strings.Join(unit.WantedBy, " "))
	builder.WriteString("\n")
	return builder.String()
}

func contractServiceNamesInStartupOrder(contract ServiceContract) []string {
	services := servicesInStartupOrder(contract.Services)
	names := make([]string, 0, len(services))
	for _, service := range services {
		names = append(names, strings.TrimSpace(service.Name))
	}
	return names
}

func servicesInStartupOrder(services []ServiceDefinition) []ServiceDefinition {
	ordered := append([]ServiceDefinition{}, services...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].StartupOrder == ordered[j].StartupOrder {
			return strings.TrimSpace(ordered[i].Name) < strings.TrimSpace(ordered[j].Name)
		}
		return ordered[i].StartupOrder < ordered[j].StartupOrder
	})
	return ordered
}

func debianUnitName(serviceName string) string {
	return "diagon-" + strings.TrimSpace(serviceName) + ".service"
}

func reverseCopy(values []string) []string {
	reversed := append([]string{}, values...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}
