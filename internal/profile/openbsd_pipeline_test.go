package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenBSDWorkflowExistsAndIsSeparatePipeline(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	workflowPath := filepath.Join(repoRoot, ".github", "workflows", "openbsd.yml")
	contents := string(mustReadFile(t, workflowPath))

	if !strings.Contains(contents, "name: OpenBSD CI") {
		t.Fatal("openbsd workflow must declare a dedicated OpenBSD CI name")
	}

	// It must be a separate pipeline from the Debian workflow.
	debianPath := filepath.Join(repoRoot, ".github", "workflows", "build.yml")
	if workflowPath == debianPath {
		t.Fatal("OpenBSD pipeline must be a separate workflow file from the Debian pipeline")
	}

	for _, job := range []string{
		"prepare-openbsd-matrix:",
		"validate-shared-config:",
		"build-openbsd-image:",
		"release-openbsd-bundle:",
		"quality-gate-openbsd:",
	} {
		if !strings.Contains(contents, job) {
			t.Fatalf("openbsd workflow must define job %q", job)
		}
	}

	for _, script := range []string{
		"bash .github/scripts/build-openbsd-matrix.sh",
		"bash .github/scripts/build-openbsd-image.sh",
		"bash .github/scripts/validate-contract-fixtures.sh",
	} {
		if !strings.Contains(contents, script) {
			t.Fatalf("openbsd workflow must invoke %q", script)
		}
	}

	if !strings.Contains(contents, "build/openbsd/images/*.img") {
		t.Fatal("openbsd workflow must publish the built disk image artifact")
	}
}

func TestOpenBSDPipelineReusesSharedDebianConfig(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	contents := string(mustReadFile(t, filepath.Join(repoRoot, ".github", "workflows", "openbsd.yml")))

	// Reuse of shared resources: same profile, policy, and service contract as Debian.
	for _, shared := range []string{
		"--profile-dir profiles",
		"--profile-name myprofile",
		"--policy-file profiles/validation-policy.json",
		"profiles/local-single-host-bootstrap.json",
	} {
		if !strings.Contains(contents, shared) {
			t.Fatalf("openbsd workflow must reuse shared Debian config %q", shared)
		}
	}
}

func TestOpenBSDReferencedScriptsExist(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	for _, rel := range []string{
		".github/scripts/build-openbsd-matrix.sh",
		".github/scripts/build-openbsd-image.sh",
		"profiles/openbsd/install.conf",
		"profiles/openbsd/install.site",
		"profiles/openbsd/pkg.list",
		".github/openbsd-matrix.json",
	} {
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected OpenBSD pipeline file %s to exist: %v", rel, err)
		}
	}
}

func TestOpenBSDMatrixFileIsWellFormed(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	raw := mustReadFile(t, filepath.Join(repoRoot, ".github", "openbsd-matrix.json"))

	var matrix struct {
		SchemaVersion int `json:"schema_version"`
		Environments  []struct {
			Environment    string `json:"environment"`
			OpenBSDVersion string `json:"openbsd_version"`
			OpenBSDArch    string `json:"openbsd_arch"`
			OpenBSDMirror  string `json:"openbsd_mirror"`
			GoVersion      string `json:"go_version"`
			Components     struct {
				Store struct {
					Repo string `json:"repo"`
				} `json:"store"`
				Paywall struct {
					Repo string `json:"repo"`
				} `json:"paywall"`
				I2pd struct {
					Repo string `json:"repo"`
				} `json:"i2pd"`
			} `json:"components"`
			ContractFixtures struct {
				Primary string `json:"primary"`
			} `json:"contract_fixtures"`
		} `json:"environments"`
	}

	if err := json.Unmarshal(raw, &matrix); err != nil {
		t.Fatalf("openbsd-matrix.json must be valid JSON: %v", err)
	}

	if len(matrix.Environments) == 0 {
		t.Fatal("openbsd-matrix.json must declare at least one environment")
	}

	for _, env := range matrix.Environments {
		if env.OpenBSDVersion == "" || env.OpenBSDArch == "" || env.OpenBSDMirror == "" {
			t.Fatalf("environment %q must pin openbsd_version, openbsd_arch, and openbsd_mirror", env.Environment)
		}
		// Shared component definitions with the Debian matrix.
		if env.Components.Store.Repo != "opd-ai/store" || env.Components.Paywall.Repo != "opd-ai/paywall" {
			t.Fatalf("environment %q must reuse the shared Store/Paywall component repos", env.Environment)
		}
		if env.ContractFixtures.Primary != "profiles/service-contract.json" {
			t.Fatalf("environment %q must reuse the shared primary service contract", env.Environment)
		}
	}
}
