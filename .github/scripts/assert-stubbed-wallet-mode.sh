#!/usr/bin/env bash
# Asserts that the emitted smoke plan and smoke harness output both report the
# stubbed wallet mode expected for CI.
set -euo pipefail

jq -e '.wallet_mode == "stubbed"' artifacts/stage-6-smoke-plan.json >/dev/null
jq -e '.initial.wallet_mode == "stubbed"' artifacts/stage-6-smoke.json >/dev/null
