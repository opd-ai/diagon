package profile

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type DebianDependencyManifest struct {
	Environment                 string                             `json:"environment"`
	DebianVersion               string                             `json:"debian_version"`
	DebianCodename              string                             `json:"debian_codename"`
	PackageDependencies         []DebianPackageDependency          `json:"package_dependencies"`
	ComponentPackageConstraints []DebianComponentPackageConstraint `json:"component_package_constraints,omitempty"`
	VerificationChecks          []string                           `json:"verification_checks"`
}

type DebianPackageDependency struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

type DebianComponentPackageConstraint struct {
	Component string `json:"component"`
	Package   string `json:"package"`
	Version   string `json:"version"`
	Codename  string `json:"codename"`
	Source    string `json:"source"`
}

func BuildDebianDependencyManifest(matrix IntegrationMatrix, environmentName string) (DebianDependencyManifest, error) {
	environment, err := matrix.EnvironmentByName(environmentName)
	if err != nil {
		return DebianDependencyManifest{}, err
	}

	if strings.TrimSpace(environment.DebianVersion) == "" || strings.TrimSpace(environment.DebianCodename) == "" {
		return DebianDependencyManifest{}, fmt.Errorf("integration environment %q must define debian_version and debian_codename", environment.Environment)
	}

	if len(environment.PackageDependencies) == 0 {
		return DebianDependencyManifest{}, fmt.Errorf("integration environment %q must define package_dependencies", environment.Environment)
	}

	packageDependencies := make([]DebianPackageDependency, 0, len(environment.PackageDependencies))
	seenPackages := make(map[string]struct{}, len(environment.PackageDependencies))
	for _, dependency := range environment.PackageDependencies {
		trimmedDependency := strings.TrimSpace(dependency)
		if trimmedDependency == "" {
			return DebianDependencyManifest{}, fmt.Errorf("integration environment %q contains empty package_dependencies entry", environment.Environment)
		}
		if _, exists := seenPackages[trimmedDependency]; exists {
			continue
		}
		seenPackages[trimmedDependency] = struct{}{}
		packageDependencies = append(packageDependencies, DebianPackageDependency{
			Name:   trimmedDependency,
			Source: "integration_matrix.package_dependencies",
		})
	}
	sort.Slice(packageDependencies, func(i, j int) bool {
		return packageDependencies[i].Name < packageDependencies[j].Name
	})

	componentConstraints := make([]DebianComponentPackageConstraint, 0, 1)
	i2pdInput := strings.TrimSpace(environment.Components.I2PD.BuildInput)
	if i2pdInput == "" {
		return DebianDependencyManifest{}, fmt.Errorf("integration environment %q component %q must define build_input", environment.Environment, "i2pd")
	}

	codename, packageName, packageVersion, parseErr := parseDebianPackageBuildInput(i2pdInput)
	if parseErr != nil {
		return DebianDependencyManifest{}, fmt.Errorf("integration environment %q component %q: %w", environment.Environment, "i2pd", parseErr)
	}
	if codename != strings.TrimSpace(environment.DebianCodename) {
		return DebianDependencyManifest{}, fmt.Errorf("integration environment %q component %q build_input codename %q does not match debian_codename %q", environment.Environment, "i2pd", codename, strings.TrimSpace(environment.DebianCodename))
	}
	componentConstraints = append(componentConstraints, DebianComponentPackageConstraint{
		Component: "i2pd",
		Package:   packageName,
		Version:   packageVersion,
		Codename:  codename,
		Source:    "integration_matrix.components.i2pd.build_input",
	})

	return DebianDependencyManifest{
		Environment:                 strings.TrimSpace(environment.Environment),
		DebianVersion:               strings.TrimSpace(environment.DebianVersion),
		DebianCodename:              strings.TrimSpace(environment.DebianCodename),
		PackageDependencies:         packageDependencies,
		ComponentPackageConstraints: componentConstraints,
		VerificationChecks: []string{
			"Install all package_dependencies on Debian using apt-get install.",
			"Verify each package dependency is installed via dpkg-query status checks.",
			"Verify each component package constraint codename aligns with debian_codename.",
		},
	}, nil
}

func WriteDebianDependencyManifest(path string, manifest DebianDependencyManifest) error {
	return writeJSONFile(path, manifest, "debian dependency manifest")
}

func parseDebianPackageBuildInput(buildInput string) (codename string, packageName string, packageVersion string, err error) {
	trimmedInput := strings.TrimSpace(buildInput)
	if trimmedInput == "" {
		return "", "", "", fmt.Errorf("build_input cannot be empty")
	}

	pattern := regexp.MustCompile(`^debian:([^/]+)/([^=]+)=([^\s]+)$`)
	matches := pattern.FindStringSubmatch(trimmedInput)
	if len(matches) != 4 {
		return "", "", "", fmt.Errorf("build_input %q must match debian:<codename>/<package>=<version>", trimmedInput)
	}

	return strings.TrimSpace(matches[1]), strings.TrimSpace(matches[2]), strings.TrimSpace(matches[3]), nil
}
