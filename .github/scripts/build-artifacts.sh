#!/usr/bin/env bash
# Builds the Diagon, Store, and Paywall artifacts inside a pinned Debian Go
# container. Requires GO_VERSION, DEBIAN_CODENAME, STORE_BUILD_INPUT, and
# PAYWALL_BUILD_INPUT in the environment.
set -euo pipefail

mkdir -p artifacts/bin
docker run --rm \
  -e STORE_BUILD_INPUT="$STORE_BUILD_INPUT" \
  -e PAYWALL_BUILD_INPUT="$PAYWALL_BUILD_INPUT" \
  -v "$PWD":/workspace \
  -w /workspace \
  "golang:${GO_VERSION}-${DEBIAN_CODENAME}" \
  bash -lc '
    set -euo pipefail
    mkdir -p artifacts/bin
    go build -o artifacts/bin/diagonctl ./cmd/diagonctl
    GOBIN=/workspace/artifacts/bin go install "$STORE_BUILD_INPUT"
    GOBIN=/workspace/artifacts/bin go install "$PAYWALL_BUILD_INPUT"
  '

test -x artifacts/bin/diagonctl
test -x artifacts/bin/store
test -x artifacts/bin/paywall
