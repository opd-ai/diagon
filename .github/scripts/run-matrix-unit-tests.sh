#!/usr/bin/env bash
# Runs readonly unit tests across Diagon plus every pinned Go component in
# COMPONENTS_JSON and asserts lockfiles stay clean (i2pd is skipped).
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

assert_lockfiles_clean() {
  local name="$1"
  local path="$2"
  local dirty=0

  pushd "$path" >/dev/null
  for lockfile in go.mod go.sum; do
    if [[ -f "$lockfile" ]] && ! git diff --exit-code -- "$lockfile" >/dev/null; then
      echo "::error::component ${name} modified ${lockfile} during readonly test execution"
      dirty=1
    fi
  done
  popd >/dev/null

  [[ "$dirty" -eq 0 ]]
}

run_go_tests() {
  local name="$1"
  local path="$2"
  shift 2
  local extra_args=("$@")

  pushd "$path" >/dev/null
  GOFLAGS=-mod=readonly go test "${extra_args[@]}" ./...
  popd >/dev/null
  assert_lockfiles_clean "$name" "$path"
}

# Non-hermetic upstream tests that reach external networks and do not honor the
# -short flag. Diagon runs pinned third-party components in -short mode and skips
# these so its own CI stays reproducible and flake-free; the components' own CI
# owns exercising them against live infrastructure.
NON_HERMETIC_TESTS='TestBitcoinTimestampProvider_GetLatestBlockTime'

component_root="$RUNNER_TEMP/matrix-components"
mkdir -p "$component_root"

while IFS=$'\t' read -r name repo version; do
  if [[ "$name" == "i2pd" ]]; then
    continue
  fi

  if [[ "$name" == "diagon" ]]; then
    # First-party: run the full, hermetic unit-test suite.
    run_go_tests "$name" "$PWD"
    continue
  fi

  destination="$component_root/$name"
  checkout_component "$name" "$repo" "$version" "$destination"
  # Third-party: run unit tests in -short mode against the locked dependency
  # state, skipping known non-hermetic network tests.
  run_go_tests "$name" "$destination" -short -skip "$NON_HERMETIC_TESTS"
done < <(printf '%s' "$COMPONENTS_JSON" | jq -r 'to_entries[] | [.key, .value.repo, .value.version] | @tsv')
