#!/usr/bin/env bash
# Verifies the emitted Debian dependency manifest matches the target codename,
# installs the declared dependencies, and enforces the i2pd codename constraint.
# Requires MATRIX_CODENAME in the environment.
set -euo pipefail

manifest=artifacts/debian-dependency-manifest.json
expected_codename=$(jq -r '.debian_codename' "$manifest")
if [[ "$expected_codename" != "$MATRIX_CODENAME" ]]; then
  echo "dependency manifest codename mismatch: expected ${MATRIX_CODENAME}, got ${expected_codename}"
  exit 1
fi

mapfile -t deps < <(jq -r '.package_dependencies[].name' "$manifest")
if [[ "${#deps[@]}" -eq 0 ]]; then
  echo "dependency manifest contains no package dependencies"
  exit 1
fi

apt-get install -y "${deps[@]}"

for dep in "${deps[@]}"; do
  dpkg-query -W -f='${Status} ${Version}\n' "$dep" | grep -q '^install ok installed '
done

jq -e --arg codename "$MATRIX_CODENAME" \
  '.component_package_constraints[] | select(.component == "i2pd") | .codename == $codename' \
  "$manifest" >/dev/null
