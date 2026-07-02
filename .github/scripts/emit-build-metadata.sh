#!/usr/bin/env bash
# Emits build metadata JSON for the current matrix environment. Requires
# ENVIRONMENT, DEBIAN_VERSION, DEBIAN_CODENAME, GO_VERSION, PACKAGE_DEPENDENCIES
# (JSON array), and COMPONENTS_JSON in the environment.
set -euo pipefail

mkdir -p artifacts/metadata

jq -n \
  --arg environment "$ENVIRONMENT" \
  --arg debian_version "$DEBIAN_VERSION" \
  --arg debian_codename "$DEBIAN_CODENAME" \
  --arg build_image "golang:${GO_VERSION}-${DEBIAN_CODENAME}" \
  --argjson package_dependencies "$PACKAGE_DEPENDENCIES" \
  --argjson components "$COMPONENTS_JSON" \
  '{
    environment: $environment,
    debian_version: $debian_version,
    debian_codename: $debian_codename,
    package_dependencies: $package_dependencies,
    components: {
      diagon: {
        repo: $components.diagon.repo,
        version: $components.diagon.version,
        build_input: $components.diagon.build_input,
        artifact: "artifacts/bin/diagonctl"
      },
      store: {
        repo: $components.store.repo,
        version: $components.store.version,
        build_input: $components.store.build_input,
        artifact: "artifacts/bin/store"
      },
      paywall: {
        repo: $components.paywall.repo,
        version: $components.paywall.version,
        build_input: $components.paywall.build_input,
        artifact: "artifacts/bin/paywall"
      },
      i2pd: {
        repo: $components.i2pd.repo,
        version: $components.i2pd.version,
        build_input: $components.i2pd.build_input,
        artifact: "debian-package"
      }
    },
    build_image: $build_image
  }' > artifacts/metadata/build-metadata.json
