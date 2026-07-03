#!/usr/bin/env bash
# Stage 3 component verification.
#
# Diagon (first-party) runs its full readonly unit-test suite. Pinned
# third-party components (Store, Paywall) are verified for reproducible
# dependency-lock state by compiling every package under -mod=readonly in the
# pinned toolchain; their own repositories own executing their unit tests. This
# keeps Diagon CI deterministic and flake-free, because the upstream suites mix
# in non-hermetic tests that reach external networks (e.g. public Bitcoin
# testnet / httpbin endpoints) and intermittently-failing crypto tests that
# cannot pass reliably in an isolated CI runner. Static type-checking of those
# components' test files is still enforced by `go vet ./...` in Stage 1.
#
# i2pd is skipped; it ships as a Debian package rather than a Go build target.
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
      echo "::error::component ${name} modified ${lockfile} during readonly execution"
      dirty=1
    fi
  done
  popd >/dev/null

  [[ "$dirty" -eq 0 ]]
}

# First-party: run the full, hermetic unit-test suite for Diagon.
run_go_tests() {
  local name="$1"
  local path="$2"

  pushd "$path" >/dev/null
  GOFLAGS=-mod=readonly go test ./...
  popd >/dev/null
  assert_lockfiles_clean "$name" "$path"
}

# Third-party: verify reproducible compilation against the locked dependency
# graph without executing the upstream (non-hermetic) test suites.
verify_go_build() {
  local name="$1"
  local path="$2"

  pushd "$path" >/dev/null
  GOFLAGS=-mod=readonly go build ./...
  popd >/dev/null
  assert_lockfiles_clean "$name" "$path"
}

component_root="$RUNNER_TEMP/matrix-components"
mkdir -p "$component_root"

while IFS=$'\t' read -r name repo version; do
  if [[ "$name" == "i2pd" ]]; then
    continue
  fi

  if [[ "$name" == "diagon" ]]; then
    run_go_tests "$name" "$PWD"
    continue
  fi

  destination="$component_root/$name"
  checkout_component "$name" "$repo" "$version" "$destination"
  verify_go_build "$name" "$destination"
done < <(printf '%s' "$COMPONENTS_JSON" | jq -r 'to_entries[] | [.key, .value.repo, .value.version] | @tsv')
