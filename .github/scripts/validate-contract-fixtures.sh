#!/usr/bin/env bash
# Validates the profile against the primary service contract plus every matrix
# fixture. Requires PRIMARY and CONTRACTS_JSON (JSON array) in the environment.
set -euo pipefail

mkdir -p artifacts/contract-tests
mapfile -t fixtures < <(
  jq -rn \
    --arg primary "$PRIMARY" \
    --argjson contracts "$CONTRACTS_JSON" \
    '([$primary] + $contracts | unique[])[]'
)
for fixture in "${fixtures[@]}"; do
  name=$(basename "$fixture" .json)
  go run ./cmd/diagonctl \
    --profile-dir profiles \
    --profile-name myprofile \
    --policy-file profiles/validation-policy.json \
    --service-contract-file "$fixture" \
    --format json | tee "artifacts/contract-tests/${name}.json"
done
