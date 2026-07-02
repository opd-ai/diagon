package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIntegrationMatrixServiceContractFixturesAreValid(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	matrixPath := filepath.Join(repoRoot, ".github", "integration-matrix.json")

	matrix, err := LoadIntegrationMatrix(matrixPath)
	if err != nil {
		t.Fatalf("LoadIntegrationMatrix() returned error: %v", err)
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

		for name, component := range map[string]IntegrationMatrixComponent{
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

		if !strings.HasPrefix(env.Components.Diagon.BuildInput, "./") {
			t.Fatalf("environment[%d].components.diagon.build_input must use a local package path, got %q", envIdx, env.Components.Diagon.BuildInput)
		}

		for _, component := range []struct {
			name      string
			component IntegrationMatrixComponent
		}{
			{name: "store", component: env.Components.Store},
			{name: "paywall", component: env.Components.Paywall},
		} {
			expectedPrefix := fmt.Sprintf("github.com/%s/", component.component.Repo)
			expectedSuffix := fmt.Sprintf("@%s", component.component.Version)
			if !strings.HasPrefix(component.component.BuildInput, expectedPrefix) {
				t.Fatalf("environment[%d].components.%s.build_input must start with %q, got %q", envIdx, component.name, expectedPrefix, component.component.BuildInput)
			}
			if !strings.HasSuffix(component.component.BuildInput, expectedSuffix) {
				t.Fatalf("environment[%d].components.%s.build_input must end with %q, got %q", envIdx, component.name, expectedSuffix, component.component.BuildInput)
			}
		}

		expectedI2PDBuildInputPrefix := fmt.Sprintf("debian:%s/i2pd=", env.DebianCodename)
		if !strings.HasPrefix(env.Components.I2PD.BuildInput, expectedI2PDBuildInputPrefix) {
			t.Fatalf("environment[%d].components.i2pd.build_input must start with %q, got %q", envIdx, expectedI2PDBuildInputPrefix, env.Components.I2PD.BuildInput)
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
