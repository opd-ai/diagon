#!/usr/bin/env bash
# Validates the emitted Debian fallback compose/service bundle content.
set -euo pipefail

bundle=artifacts/debian-compose-bundle.json
jq -e '.bundle_name == "debian-compose-fallback"' "$bundle" >/dev/null
jq -e '.compose.path == "/opt/diagon/compose/compose.yaml"' "$bundle" >/dev/null
jq -e '.systemd_unit.path == "/etc/systemd/system/diagon-compose.service"' "$bundle" >/dev/null
jq -e '.environment_template.path == "/etc/diagon/compose/diagon-compose.env.example"' "$bundle" >/dev/null
jq -e '.manual_install_steps | length >= 5' "$bundle" >/dev/null
jq -r '.compose.content' "$bundle" | grep -q 'container_name: diagon-i2pd'
jq -r '.compose.content' "$bundle" | grep -q 'container_name: diagon-paywall'
jq -r '.compose.content' "$bundle" | grep -q 'container_name: diagon-store'
jq -r '.systemd_unit.content' "$bundle" | grep -q 'ExecStart=/usr/bin/docker compose'
jq -r '.manual_install_guide.content' "$bundle" | grep -q 'systemctl enable --now diagon-compose.service'
