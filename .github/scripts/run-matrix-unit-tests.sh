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

  pushd "$path" >/dev/null
  GOFLAGS=-mod=readonly go test ./...
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
  run_go_tests "$name" "$destination"
done < <(printf '%s' "$COMPONENTS_JSON" | jq -r 'to_entries[] | [.key, .value.repo, .value.version] | @tsv')
