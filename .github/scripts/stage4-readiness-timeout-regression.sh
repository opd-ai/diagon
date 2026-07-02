#!/usr/bin/env bash
# Kills the Paywall stub and asserts that live readiness probing fails with a
# clear timeout error. Requires SERVICE_CONTRACT_PRIMARY in the environment.
set -euo pipefail

kill "$(cat .tmp/paywall.pid)"
rm -f .tmp/paywall.pid

if go run ./cmd/diagonctl \
  --profile-dir profiles \
  --profile-name myprofile \
  --policy-file profiles/validation-policy.json \
  --bootstrap-profile-file profiles/local-single-host-bootstrap.json \
  --service-contract-file "$SERVICE_CONTRACT_PRIMARY" \
  --probe-live \
  --probe-timeout 1s \
  --probe-interval 100ms \
  --format json > artifacts/service-probe-timeout.json; then
  echo "expected readiness-timeout probe to fail when paywall is unavailable"
  exit 1
fi

jq -e '.status == "failed"' artifacts/service-probe-timeout.json >/dev/null
jq -e '.probe_live == true' artifacts/service-probe-timeout.json >/dev/null
jq -e '.aggregated_health.ready == false' artifacts/service-probe-timeout.json >/dev/null
jq -e 'any(.errors[]; contains("service \"paywall\" failed runtime readiness probe within 1s"))' artifacts/service-probe-timeout.json >/dev/null
