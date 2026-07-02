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
	ContractFixtures integrationContractFixtures `json:"contract_fixtures"`
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

func mustRepoRoot(t *testing.T) string {
	t.Helper()

	_, currentFilePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFilePath), "..", ".."))
}
