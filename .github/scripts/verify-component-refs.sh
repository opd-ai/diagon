#!/usr/bin/env bash
# Verifies that every pinned component ref in COMPONENTS_JSON resolves to a real
# branch or tag in its upstream repository.
set -euo pipefail

verify_ref() {
  local name="$1"
  local repo="$2"
  local version="$3"
  local url="https://github.com/${repo}.git"

  if git ls-remote --exit-code --heads "$url" "$version" >/dev/null 2>&1; then
    return 0
  fi
  if git ls-remote --exit-code --tags "$url" "refs/tags/$version" >/dev/null 2>&1; then
    return 0
  fi
  if git ls-remote --exit-code --tags "$url" "refs/tags/$version^{}" >/dev/null 2>&1; then
    return 0
  fi

  echo "::error::component ${name} could not resolve pinned ref ${version} in ${repo}"
  return 1
}

while IFS=$'\t' read -r name repo version; do
  verify_ref "$name" "$repo" "$version"
done < <(printf '%s' "$COMPONENTS_JSON" | jq -r 'to_entries[] | [.key, .value.repo, .value.version] | @tsv')
