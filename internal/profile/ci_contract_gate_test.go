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

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return contents
}
