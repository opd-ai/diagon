package profile

import (
	"os"
	"path/filepath"
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

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
