package profile

import (
	"os"
	"path/filepath"
	"regexp"
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

	if !strings.Contains(contents, "bash .github/scripts/validate-contract-fixtures.sh") {
		t.Fatal("stage-5-contract-tests must run validate-contract-fixtures.sh")
	}

	scriptPath := filepath.Join(repoRoot, ".github", "scripts", "validate-contract-fixtures.sh")
	script := string(mustReadFile(t, scriptPath))
	if !strings.Contains(script, "--service-contract-file \"$fixture\" \\") {
		t.Fatal("validate-contract-fixtures.sh must execute diagonctl for each fixture")
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

	if !strings.Contains(contents, "bash .github/scripts/verify-debian-toolchain.sh") {
		t.Fatal("stage-7-packaging-verification must run verify-debian-toolchain.sh")
	}

	toolchainScript := string(mustReadFile(t, filepath.Join(repoRoot, ".github", "scripts", "verify-debian-toolchain.sh")))
	if !strings.Contains(toolchainScript, "jq -r '.package_dependencies[].name' \"$manifest\"") {
		t.Fatal("verify-debian-toolchain.sh must install dependencies from emitted manifest")
	}

	if !strings.Contains(toolchainScript, ".component_package_constraints[] | select(.component == \"i2pd\") | .codename == $codename") {
		t.Fatal("verify-debian-toolchain.sh must enforce i2pd codename constraint from emitted manifest")
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

	if !strings.Contains(contents, "bash .github/scripts/assert-stubbed-wallet-mode.sh") {
		t.Fatal("stage-6-e2e-smoke must run assert-stubbed-wallet-mode.sh")
	}

	walletScript := string(mustReadFile(t, filepath.Join(repoRoot, ".github", "scripts", "assert-stubbed-wallet-mode.sh")))
	if !strings.Contains(walletScript, "jq -e '.wallet_mode == \"stubbed\"' artifacts/stage-6-smoke-plan.json") {
		t.Fatal("assert-stubbed-wallet-mode.sh must assert stubbed wallet mode in emitted smoke plan")
	}

	if !strings.Contains(walletScript, "jq -e '.initial.wallet_mode == \"stubbed\"' artifacts/stage-6-smoke.json") {
		t.Fatal("assert-stubbed-wallet-mode.sh must assert stubbed wallet mode in smoke harness output")
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

	if !strings.Contains(contents, "bash .github/scripts/validate-fallback-bundle.sh") {
		t.Fatal("stage-7b fallback job must run validate-fallback-bundle.sh")
	}

	fallbackScript := string(mustReadFile(t, filepath.Join(repoRoot, ".github", "scripts", "validate-fallback-bundle.sh")))
	if !strings.Contains(fallbackScript, "jq -e '.compose.path == \"/opt/diagon/compose/compose.yaml\"'") {
		t.Fatal("validate-fallback-bundle.sh must assert the emitted compose path")
	}

	if !strings.Contains(fallbackScript, "jq -e '.systemd_unit.path == \"/etc/systemd/system/diagon-compose.service\"'") {
		t.Fatal("validate-fallback-bundle.sh must assert the emitted systemd unit path")
	}

	if !strings.Contains(contents, "- stage-7b-fallback-compose-bundle") {
		t.Fatal("merge quality gate must depend on stage-7b-fallback-compose-bundle")
	}
}

func TestBuildWorkflowHasNoEmbeddedPythonAndScriptsExist(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	workflowPath := filepath.Join(repoRoot, ".github", "workflows", "build.yml")
	contents := string(mustReadFile(t, workflowPath))

	if strings.Contains(contents, "python3") || strings.Contains(contents, "<<'PY'") {
		t.Fatal("build workflow must not embed inline Python programs; extract them to Go tools or scripts")
	}

	re := regexp.MustCompile(`bash (\.github/scripts/[A-Za-z0-9._-]+\.sh)`)
	matches := re.FindAllStringSubmatch(contents, -1)
	if len(matches) == 0 {
		t.Fatal("build workflow must invoke extracted scripts under .github/scripts")
	}
	for _, match := range matches {
		scriptPath := filepath.Join(repoRoot, filepath.FromSlash(match[1]))
		if _, err := os.Stat(scriptPath); err != nil {
			t.Fatalf("referenced script %s does not exist: %v", match[1], err)
		}
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
