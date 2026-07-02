package profile

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDebianDependencyManifestFromMatrix(t *testing.T) {
	t.Parallel()

	matrix, err := LoadIntegrationMatrix(filepath.Join("..", "..", ".github", "integration-matrix.json"))
	if err != nil {
		t.Fatalf("LoadIntegrationMatrix() returned error: %v", err)
	}

	manifest, err := BuildDebianDependencyManifest(matrix, "debian-12")
	if err != nil {
		t.Fatalf("BuildDebianDependencyManifest() returned error: %v", err)
	}

	if manifest.Environment != "debian-12" {
		t.Fatalf("expected environment debian-12, got %q", manifest.Environment)
	}
	if manifest.DebianVersion != "12" {
		t.Fatalf("expected debian version 12, got %q", manifest.DebianVersion)
	}
	if manifest.DebianCodename != "bookworm" {
		t.Fatalf("expected debian codename bookworm, got %q", manifest.DebianCodename)
	}
	if len(manifest.PackageDependencies) < 2 {
		t.Fatalf("expected at least 2 package dependencies, got %d", len(manifest.PackageDependencies))
	}

	foundSimpleCDD := false
	foundArchiveKeyring := false
	for _, dependency := range manifest.PackageDependencies {
		if dependency.Name == "simple-cdd" {
			foundSimpleCDD = true
		}
		if dependency.Name == "debian-archive-keyring" {
			foundArchiveKeyring = true
		}
		if dependency.Source != "integration_matrix.package_dependencies" {
			t.Fatalf("unexpected package dependency source: %q", dependency.Source)
		}
	}
	if !foundSimpleCDD {
		t.Fatal("expected simple-cdd in package dependency manifest")
	}
	if !foundArchiveKeyring {
		t.Fatal("expected debian-archive-keyring in package dependency manifest")
	}

	if len(manifest.ComponentPackageConstraints) != 1 {
		t.Fatalf("expected 1 component package constraint, got %d", len(manifest.ComponentPackageConstraints))
	}
	i2pdConstraint := manifest.ComponentPackageConstraints[0]
	if i2pdConstraint.Component != "i2pd" {
		t.Fatalf("expected i2pd constraint component, got %q", i2pdConstraint.Component)
	}
	if i2pdConstraint.Package != "i2pd" {
		t.Fatalf("expected i2pd package constraint package, got %q", i2pdConstraint.Package)
	}
	if i2pdConstraint.Codename != "bookworm" {
		t.Fatalf("expected i2pd codename constraint bookworm, got %q", i2pdConstraint.Codename)
	}
	if i2pdConstraint.Version != "2.54.0-1" {
		t.Fatalf("expected i2pd version constraint 2.54.0-1, got %q", i2pdConstraint.Version)
	}
	if i2pdConstraint.Source != "integration_matrix.components.i2pd.build_input" {
		t.Fatalf("unexpected i2pd constraint source: %q", i2pdConstraint.Source)
	}

	if len(manifest.VerificationChecks) != 3 {
		t.Fatalf("expected 3 verification checks, got %d", len(manifest.VerificationChecks))
	}
}

func TestBuildDebianDependencyManifestRejectsMismatchedI2PDCodename(t *testing.T) {
	t.Parallel()

	matrix, err := LoadIntegrationMatrix(filepath.Join("..", "..", ".github", "integration-matrix.json"))
	if err != nil {
		t.Fatalf("LoadIntegrationMatrix() returned error: %v", err)
	}

	matrix.Environments[0].Components.I2PD.BuildInput = "debian:trixie/i2pd=2.54.0-1"

	_, err = BuildDebianDependencyManifest(matrix, matrix.Environments[0].Environment)
	if err == nil {
		t.Fatal("expected BuildDebianDependencyManifest() to fail on codename mismatch")
	}
	if !strings.Contains(err.Error(), "does not match debian_codename") {
		t.Fatalf("expected codename mismatch error, got %v", err)
	}
}

func TestWriteDebianDependencyManifestRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	err := WriteDebianDependencyManifest("", DebianDependencyManifest{})
	if err == nil {
		t.Fatal("expected empty output path to fail")
	}
}
