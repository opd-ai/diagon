package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type integrationMatrix struct {
	Environments []integrationEnvironment `json:"environments"`
}

type integrationEnvironment struct {
	Environment         string                      `json:"environment"`
	DebianVersion       string                      `json:"debian_version"`
	DebianCodename      string                      `json:"debian_codename"`
	PackageDependencies []string                    `json:"package_dependencies"`
	Components          integrationMatrixComponents `json:"components"`
	ContractFixtures    integrationContractFixtures `json:"contract_fixtures"`
}

type integrationMatrixComponents struct {
	Diagon  integrationMatrixComponent `json:"diagon"`
	Store   integrationMatrixComponent `json:"store"`
	Paywall integrationMatrixComponent `json:"paywall"`
	I2PD    integrationMatrixComponent `json:"i2pd"`
}

type integrationMatrixComponent struct {
	Repo       string `json:"repo"`
	Version    string `json:"version"`
	BuildInput string `json:"build_input"`
}

type integrationContractFixtures struct {
	Primary          string   `json:"primary"`
	ServiceContracts []string `json:"service_contracts"`
}

func TestIntegrationMatrixServiceContractFixturesAreValid(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	matrixPath := filepath.Join(repoRoot, ".github", "integration-matrix.json")

	raw, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatalf("read integration matrix: %v", err)
	}

	var matrix integrationMatrix
	if err := json.Unmarshal(raw, &matrix); err != nil {
		t.Fatalf("parse integration matrix: %v", err)
	}

	if len(matrix.Environments) == 0 {
		t.Fatal("integration matrix must define at least one environment")
	}

	for envIdx, env := range matrix.Environments {
		if env.Environment == "" {
			t.Fatalf("environment[%d] must define environment", envIdx)
		}
		if env.DebianVersion == "" {
			t.Fatalf("environment[%d] must define debian_version", envIdx)
		}
		if env.DebianCodename == "" {
			t.Fatalf("environment[%d] must define debian_codename", envIdx)
		}
		if len(env.PackageDependencies) == 0 {
			t.Fatalf("environment[%d] must define package_dependencies", envIdx)
		}
		if !contains(env.PackageDependencies, "simple-cdd") {
			t.Fatalf("environment[%d] package_dependencies must include simple-cdd", envIdx)
		}
		if !contains(env.PackageDependencies, "debian-archive-keyring") {
			t.Fatalf("environment[%d] package_dependencies must include debian-archive-keyring", envIdx)
		}

		for name, component := range map[string]integrationMatrixComponent{
			"diagon":  env.Components.Diagon,
			"store":   env.Components.Store,
			"paywall": env.Components.Paywall,
			"i2pd":    env.Components.I2PD,
		} {
			if component.Repo == "" {
				t.Fatalf("environment[%d].components.%s.repo must be set", envIdx, name)
			}
			if component.Version == "" {
				t.Fatalf("environment[%d].components.%s.version must be set", envIdx, name)
			}
			if component.BuildInput == "" {
				t.Fatalf("environment[%d].components.%s.build_input must be set", envIdx, name)
			}
		}

		if env.ContractFixtures.Primary == "" {
			t.Fatalf("environment[%d] must define contract_fixtures.primary", envIdx)
		}
		fixturePaths := append([]string{env.ContractFixtures.Primary}, env.ContractFixtures.ServiceContracts...)
		if len(fixturePaths) == 0 {
			t.Fatalf("environment[%d] must define at least one service contract fixture", envIdx)
		}

		for _, fixtureRelPath := range fixturePaths {
			fixturePath := filepath.Join(repoRoot, filepath.FromSlash(fixtureRelPath))
			if _, err := os.Stat(fixturePath); err != nil {
				t.Fatalf("fixture path %q does not exist: %v", fixtureRelPath, err)
			}

			contract, err := LoadServiceContract(fixturePath)
			if err != nil {
				t.Fatalf("load fixture %q: %v", fixtureRelPath, err)
			}

			result := ValidateServiceContractDefinition(contract)
			if result.HasErrors() {
				t.Fatalf("fixture %q has validation errors: %v", fixtureRelPath, result.Errors)
			}
		}
	}
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}

	return false
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()

	_, currentFilePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFilePath), "..", ".."))
}
