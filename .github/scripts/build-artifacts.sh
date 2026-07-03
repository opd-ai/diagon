#!/usr/bin/env bash
# Builds the Diagon, Store, and Paywall artifacts inside a pinned Debian Go
# container.
#
# Store and Paywall are built from cloned checkouts rather than via
# `go install <module>@version`, because that install form refuses modules whose
# go.mod contains replace directives (Store pins several dependencies this way).
#
# Requires the following in the environment:
#   GO_VERSION, DEBIAN_CODENAME
#   STORE_REPO, STORE_VERSION, STORE_BUILD_INPUT
#   PAYWALL_REPO, PAYWALL_VERSION, PAYWALL_BUILD_INPUT
set -euo pipefail

: "${GO_VERSION:?GO_VERSION is required}"
: "${DEBIAN_CODENAME:?DEBIAN_CODENAME is required}"
: "${STORE_REPO:?STORE_REPO is required}"
: "${STORE_VERSION:?STORE_VERSION is required}"
: "${STORE_BUILD_INPUT:?STORE_BUILD_INPUT is required}"
: "${PAYWALL_REPO:?PAYWALL_REPO is required}"
: "${PAYWALL_VERSION:?PAYWALL_VERSION is required}"
: "${PAYWALL_BUILD_INPUT:?PAYWALL_BUILD_INPUT is required}"

# Derives the in-tree package path (e.g. ./cmd/store) from a module build input
# like github.com/opd-ai/store/cmd/store@main and its owning repo.
component_pkg() {
  local repo="$1" build_input="$2"
  local pkg="${build_input%@*}"          # strip @version
  local rel="${pkg#github.com/${repo}}"  # strip module path prefix
  rel="${rel#/}"
  if [[ -z "$rel" ]]; then
    printf '.'
  else
    printf './%s' "$rel"
  fi
}

clone_component() {
  local repo="$1" version="$2" destination="$3"

  rm -rf "$destination"
  git clone --depth 1 --branch "$version" "https://github.com/${repo}.git" "$destination" 2>/dev/null && return 0

  rm -rf "$destination"
  git clone --depth 1 "https://github.com/${repo}.git" "$destination"
  git -C "$destination" checkout "$version"
}

store_pkg="$(component_pkg "$STORE_REPO" "$STORE_BUILD_INPUT")"
paywall_pkg="$(component_pkg "$PAYWALL_REPO" "$PAYWALL_BUILD_INPUT")"

component_root="build/matrix-components"
rm -rf "$component_root"
mkdir -p "$component_root" artifacts/bin

clone_component "$STORE_REPO" "$STORE_VERSION" "$component_root/store"
clone_component "$PAYWALL_REPO" "$PAYWALL_VERSION" "$component_root/paywall"

docker run --rm \
  -e STORE_PKG="$store_pkg" \
  -e PAYWALL_PKG="$paywall_pkg" \
  -v "$PWD":/workspace \
  -w /workspace \
  "golang:${GO_VERSION}-${DEBIAN_CODENAME}" \
  bash -c '
    set -euo pipefail
    # The workspace is bind-mounted and owned by the runner user while the
    # container runs as root, so git reports "dubious ownership". Mark all
    # paths safe and disable VCS stamping defensively.
    git config --global --add safe.directory "*" 2>/dev/null || true
    mkdir -p artifacts/bin
    go build -buildvcs=false -o artifacts/bin/diagonctl ./cmd/diagonctl
    ( cd build/matrix-components/store && go build -buildvcs=false -o /workspace/artifacts/bin/store "$STORE_PKG" )
    ( cd build/matrix-components/paywall && go build -buildvcs=false -o /workspace/artifacts/bin/paywall "$PAYWALL_PKG" )
  '

test -x artifacts/bin/diagonctl
test -x artifacts/bin/store
test -x artifacts/bin/paywall
