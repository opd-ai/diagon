#!/usr/bin/env bash
# Runs gofmt and go vet across Diagon plus every pinned Go component in
# COMPONENTS_JSON (i2pd is skipped; it ships as a Debian package).
set -euo pipefail

checkout_component() {
  local name="$1"
  local repo="$2"
  local version="$3"
  local destination="$4"

  rm -rf "$destination"
  git clone --depth 1 --branch "$version" "https://github.com/${repo}.git" "$destination" 2>/dev/null && return 0

  rm -rf "$destination"
  git clone --depth 1 "https://github.com/${repo}.git" "$destination"
  git -C "$destination" checkout "$version"
}

run_go_static_checks() {
  local name="$1"
  local path="$2"

  pushd "$path" >/dev/null
  unformatted=$(gofmt -l .)
  if [[ -n "$unformatted" ]]; then
    echo "::error::component ${name} has unformatted Go files"
    echo "$unformatted"
    exit 1
  fi
  GOFLAGS=-mod=readonly go vet ./...
  popd >/dev/null
}

component_root="$RUNNER_TEMP/matrix-components"
mkdir -p "$component_root"

while IFS=$'\t' read -r name repo version; do
  if [[ "$name" == "i2pd" ]]; then
    continue
  fi

  if [[ "$name" == "diagon" ]]; then
    run_go_static_checks "$name" "$PWD"
    continue
  fi

  destination="$component_root/$name"
  checkout_component "$name" "$repo" "$version" "$destination"
  run_go_static_checks "$name" "$destination"
done < <(printf '%s' "$COMPONENTS_JSON" | jq -r 'to_entries[] | [.key, .value.repo, .value.version] | @tsv')
