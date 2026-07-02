#!/usr/bin/env bash
# Best-effort cleanup of any background stub services started during bootstrap.
set -uo pipefail

for pid_file in .tmp/*.pid; do
  if [ -f "$pid_file" ]; then
    kill "$(cat "$pid_file")" || true
  fi
done
