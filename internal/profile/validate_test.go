package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidateSuccess(t *testing.T) {
	t.Parallel()

	profileDir := t.TempDir()
	writeFile(t, filepath.Join(profileDir, "myprofile.packages"), "curl\nopenssh-server\ni2pd\n")
	writeFile(t, filepath.Join(profileDir, "myprofile.preseed"), "d-i passwd/root-login boolean false\nd-i time/zone string UTC\n")

	result, err := Validate(profileDir, "myprofile")
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
	if result.HasErrors() {
		t.Fatalf("Validate() returned errors: %v", result.Errors)
	}
}

func TestValidateMissingRequiredPackage(t *testing.T) {
	t.Parallel()

	profileDir := t.TempDir()
	writeFile(t, filepath.Join(profileDir, "myprofile.packages"), "curl\nopenssh-server\n")
	writeFile(t, filepath.Join(profileDir, "myprofile.preseed"), "d-i passwd/root-login boolean false\nd-i time/zone string UTC\n")

	result, err := Validate(profileDir, "myprofile")
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
	if !result.HasErrors() {
		t.Fatal("Validate() expected errors, got none")
	}

	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "missing required package \"i2pd\"") {
		t.Fatalf("expected missing i2pd error, got: %s", joined)
	}
}

func TestValidateInvalidRootLoginPreseed(t *testing.T) {
	t.Parallel()

	profileDir := t.TempDir()
	writeFile(t, filepath.Join(profileDir, "myprofile.packages"), "curl\nopenssh-server\ni2pd\n")
	writeFile(t, filepath.Join(profileDir, "myprofile.preseed"), "d-i passwd/root-login boolean true\nd-i time/zone string UTC\n")

	result, err := Validate(profileDir, "myprofile")
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
	if !result.HasErrors() {
		t.Fatal("Validate() expected errors, got none")
	}

	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "must be \"false\"") {
		t.Fatalf("expected root-login value error, got: %s", joined)
	}
}

func TestValidateWildcardPackageWarning(t *testing.T) {
	t.Parallel()

	profileDir := t.TempDir()
	writeFile(t, filepath.Join(profileDir, "myprofile.packages"), "curl\nopenssh-server\ni2pd\n*archive-keyring\n")
	writeFile(t, filepath.Join(profileDir, "myprofile.preseed"), "d-i passwd/root-login boolean false\nd-i time/zone string UTC\n")

	result, err := Validate(profileDir, "myprofile")
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d (%v)", len(result.Warnings), result.Warnings)
	}
	if !strings.Contains(result.Warnings[0], "wildcard package") {
		t.Fatalf("expected wildcard warning, got: %s", result.Warnings[0])
	}
}

func TestValidateWithCustomPolicy(t *testing.T) {
	t.Parallel()

	profileDir := t.TempDir()
	writeFile(t, filepath.Join(profileDir, "custom.packages"), "curl\ni2pd\n")
	writeFile(t, filepath.Join(profileDir, "custom.preseed"), "d-i passwd/root-login boolean false\nd-i time/zone string UTC\n")

	policy := Policy{
		RequiredPackages: map[string]struct{}{
			"curl": {},
			"i2pd": {},
		},
		RequiredPreseed: map[string]string{
			"passwd/root-login": "false",
			"time/zone":         "UTC",
		},
	}

	result, err := ValidateWithPolicy(profileDir, "custom", policy)
	if err != nil {
		t.Fatalf("ValidateWithPolicy() returned error: %v", err)
	}
	if result.HasErrors() {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
}

func TestValidateDuplicateEntriesWarn(t *testing.T) {
	t.Parallel()

	profileDir := t.TempDir()
	writeFile(t, filepath.Join(profileDir, "myprofile.packages"), "curl\ncurl\nopenssh-server\ni2pd\n")
	writeFile(t, filepath.Join(profileDir, "myprofile.preseed"), "d-i passwd/root-login boolean false\nd-i passwd/root-login boolean false\nd-i time/zone string UTC\n")

	result, err := Validate(profileDir, "myprofile")
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	joined := strings.Join(result.Warnings, "\n")
	if !strings.Contains(joined, "duplicate package") {
		t.Fatalf("expected duplicate package warning, got: %s", joined)
	}
	if !strings.Contains(joined, "duplicate preseed key") {
		t.Fatalf("expected duplicate preseed warning, got: %s", joined)
	}
}

func TestLoadPolicySuccess(t *testing.T) {
	t.Parallel()

	policyPath := filepath.Join(t.TempDir(), "policy.json")
	raw := map[string]any{
		"required_packages": []string{"curl", "i2pd"},
		"required_preseed": map[string]string{
			"passwd/root-login": "false",
			"time/zone":         "*non-empty*",
		},
	}
	bytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal policy json: %v", err)
	}
	writeFile(t, policyPath, string(bytes))

	policy, err := LoadPolicy(policyPath)
	if err != nil {
		t.Fatalf("LoadPolicy() returned error: %v", err)
	}

	if _, ok := policy.RequiredPackages["curl"]; !ok {
		t.Fatalf("expected required package curl in policy: %#v", policy.RequiredPackages)
	}
	if got, ok := policy.RequiredPreseed["passwd/root-login"]; !ok || got != "false" {
		t.Fatalf("unexpected required preseed map: %#v", policy.RequiredPreseed)
	}
}

func TestResultSort(t *testing.T) {
	t.Parallel()

	result := Result{
		Errors:   []string{"b", "a"},
		Warnings: []string{"2", "1"},
	}
	result.Sort()

	if !reflect.DeepEqual(result.Errors, []string{"a", "b"}) {
		t.Fatalf("unexpected error ordering: %v", result.Errors)
	}
	if !reflect.DeepEqual(result.Warnings, []string{"1", "2"}) {
		t.Fatalf("unexpected warning ordering: %v", result.Warnings)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
