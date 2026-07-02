package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWorkflowRunsContractTestsOnPushAndPullRequest(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	workflowPath := filepath.Join(repoRoot, ".github", "workflows", "build.yml")
	contents := string(mustReadFile(t, workflowPath))

	if !strings.Contains(contents, "on:\n  push:\n  pull_request:\n") {
		t.Fatal("build workflow must trigger on both push and pull_request events")
	}

	if !strings.Contains(contents, "stage-5-contract-tests:") {
		t.Fatal("build workflow must define stage-5-contract-tests job")
	}

	if !strings.Contains(contents, "name: Stage 5 - Contract tests") {
		t.Fatal("stage-5-contract-tests job must keep its contract test stage")
	}

	if !strings.Contains(contents, "quality-gate-merge:") || !strings.Contains(contents, "- stage-5-contract-tests") {
		t.Fatal("merge quality gate must depend on stage-5-contract-tests")
	}

	if !strings.Contains(contents, "--service-contract-file \"$fixture\" \\") {
		t.Fatal("stage-5-contract-tests must execute diagonctl for each fixture")
	}
}

func TestBuildWorkflowStage7UsesDebianDependencyManifest(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	workflowPath := filepath.Join(repoRoot, ".github", "workflows", "build.yml")
	contents := string(mustReadFile(t, workflowPath))

	if !strings.Contains(contents, "stage-7-packaging-verification:") {
		t.Fatal("build workflow must define stage-7-packaging-verification job")
	}

	if !strings.Contains(contents, "image: debian:${{ matrix.debian_codename }}") {
		t.Fatal("stage-7-packaging-verification must pin a Debian container image by matrix codename")
	}

	if !strings.Contains(contents, "--emit-debian-dependency-manifest-file artifacts/debian-dependency-manifest.json \\") {
		t.Fatal("stage-7-packaging-verification must emit debian dependency manifest")
	}

	if !strings.Contains(contents, "jq -r '.package_dependencies[].name' \"$manifest\"") {
		t.Fatal("stage-7-packaging-verification must install dependencies from emitted manifest")
	}

	if !strings.Contains(contents, "jq -e '.component_package_constraints[] | select(.component == \"i2pd\") | .codename == \"${{ matrix.debian_codename }}\"'") {
		t.Fatal("stage-7-packaging-verification must enforce i2pd codename constraint from emitted manifest")
	}
}

func TestBuildWorkflowIncludesWalletChecklistAndStubbedCIAssertions(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	workflowPath := filepath.Join(repoRoot, ".github", "workflows", "build.yml")
	contents := string(mustReadFile(t, workflowPath))

	if !strings.Contains(contents, "--emit-wallet-validation-checklist-file artifacts/wallet-validation-checklist.md \\") {
		t.Fatal("stage-6-e2e-smoke must emit wallet validation checklist artifact")
	}

	if !strings.Contains(contents, "--emit-bootstrap-quickstart-file artifacts/bootstrap-quickstart.md \\") {
		t.Fatal("stage-6-e2e-smoke must emit bootstrap quickstart artifact")
	}

	if !strings.Contains(contents, "jq -e '.wallet_mode == \"stubbed\"' artifacts/stage-6-smoke-plan.json") {
		t.Fatal("stage-6-e2e-smoke must assert stubbed wallet mode in emitted smoke plan")
	}

	if !strings.Contains(contents, "jq -e '.initial.wallet_mode == \"stubbed\"' artifacts/stage-6-smoke.json") {
		t.Fatal("stage-6-e2e-smoke must assert stubbed wallet mode in smoke harness output")
	}

	if !strings.Contains(contents, "--emit-wallet-validation-checklist-file release-artifacts/manifest/wallet-validation-checklist.md \\") {
		t.Fatal("stage-8-release-bundle must include emitted wallet validation checklist")
	}

	if !strings.Contains(contents, "--emit-bootstrap-quickstart-file release-artifacts/manifest/bootstrap-quickstart.md \\") {
		t.Fatal("stage-8-release-bundle must include emitted bootstrap quickstart")
	}
}

func TestBuildWorkflowIncludesFallbackComposeBundleValidation(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	workflowPath := filepath.Join(repoRoot, ".github", "workflows", "build.yml")
	contents := string(mustReadFile(t, workflowPath))

	if !strings.Contains(contents, "stage-7b-fallback-compose-bundle:") {
		t.Fatal("build workflow must define stage-7b-fallback-compose-bundle job")
	}

	if !strings.Contains(contents, "name: Stage 7b - Fallback compose bundle") {
		t.Fatal("fallback compose bundle job must keep Stage 7b naming")
	}

	if !strings.Contains(contents, "--emit-debian-compose-bundle-file artifacts/debian-compose-bundle.json \\") {
		t.Fatal("stage-7b fallback job must emit the Debian compose bundle artifact")
	}

	if !strings.Contains(contents, "jq -e '.compose.path == \"/opt/diagon/compose/compose.yaml\"'") {
		t.Fatal("stage-7b fallback job must assert the emitted compose path")
	}

	if !strings.Contains(contents, "jq -e '.systemd_unit.path == \"/etc/systemd/system/diagon-compose.service\"'") {
		t.Fatal("stage-7b fallback job must assert the emitted systemd unit path")
	}

	if !strings.Contains(contents, "- stage-7b-fallback-compose-bundle") {
		t.Fatal("merge quality gate must depend on stage-7b-fallback-compose-bundle")
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return contents
}
