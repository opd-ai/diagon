package profile

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDebianComposeServiceBundleSuccess(t *testing.T) {
	t.Parallel()

	matrix, err := LoadIntegrationMatrix(filepath.Join("..", "..", ".github", "integration-matrix.json"))
	if err != nil {
		t.Fatalf("LoadIntegrationMatrix() returned error: %v", err)
	}
	environment, err := matrix.EnvironmentByName("debian-12")
	if err != nil {
		t.Fatalf("EnvironmentByName() returned error: %v", err)
	}

	bundle, err := BuildDebianComposeServiceBundle(loadBootstrapFixture(t), loadServiceContractFixture(t), &environment)
	if err != nil {
		t.Fatalf("BuildDebianComposeServiceBundle() returned error: %v", err)
	}

	if bundle.BundleName != "debian-compose-fallback" {
		t.Fatalf("expected bundle name debian-compose-fallback, got %q", bundle.BundleName)
	}
	if bundle.Compose.Path != "/opt/diagon/compose/compose.yaml" {
		t.Fatalf("unexpected compose output path: %q", bundle.Compose.Path)
	}
	if !strings.Contains(bundle.Compose.Content, "services:") {
		t.Fatal("expected compose content to define services")
	}
	if !strings.Contains(bundle.Compose.Content, "container_name: diagon-store") {
		t.Fatal("expected compose content to include store service")
	}
	if !strings.Contains(bundle.Compose.Content, "depends_on:") {
		t.Fatal("expected compose content to include dependency ordering")
	}
	if !strings.Contains(bundle.SystemdUnit.Content, "ExecStart=/usr/bin/docker compose") {
		t.Fatal("expected systemd unit to run docker compose")
	}
	if !strings.Contains(bundle.EnvironmentTemplate.Content, "PAYWALL_WALLET_RPC_USER=<required>") {
		t.Fatal("expected env template to include paywall wallet user")
	}
	if !strings.Contains(bundle.EnvironmentTemplate.Content, "STORE_SESSION_SECRET_FILE=/run/secrets/store-session-secret") {
		t.Fatal("expected env template to include store secret file path")
	}
	if len(bundle.ValidationChecks) < 5 {
		t.Fatalf("expected validation checks for compose, services, and tunnels, got %d", len(bundle.ValidationChecks))
	}
	if bundle.PinnedImageReference["store"] != "ghcr.io/opd-ai/store:main" {
		t.Fatalf("unexpected pinned store image reference: %q", bundle.PinnedImageReference["store"])
	}
	if !strings.Contains(bundle.ManualInstallGuide.Content, "systemctl enable --now diagon-compose.service") {
		t.Fatal("expected manual guide to include service enable command")
	}
}

func TestBuildDebianComposeServiceBundleRejectsInvalidBootstrap(t *testing.T) {
	t.Parallel()

	contract := loadServiceContractFixture(t)
	bootstrap := loadBootstrapFixture(t)
	bootstrap.Components = bootstrap.Components[:2]

	if _, err := BuildDebianComposeServiceBundle(bootstrap, contract, nil); err == nil {
		t.Fatal("expected missing store component to fail")
	}
}

func TestWriteDebianComposeServiceBundleRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	if err := WriteDebianComposeServiceBundle("", DebianComposeServiceBundle{}); err == nil {
		t.Fatal("expected empty output path to fail")
	}
}
