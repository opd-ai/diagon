#!/usr/bin/env bash
# Builds the Debian installer ISO with simple-cdd and verifies the output.
# Requires MATRIX_CODENAME in the environment.
set -euo pipefail

mkdir -p build
cp -r profiles build/

# simple-cdd (debian-cd) refuses to run as root. The Stage 7 packaging job runs
# inside a debian:<codename> container as root, so build under a dedicated
# unprivileged user when necessary.
if [ "$(id -u)" -eq 0 ]; then
  if ! id builder >/dev/null 2>&1; then
    useradd -m -s /bin/bash builder
  fi
  chown -R builder:builder build
  runuser -u builder -- env MATRIX_CODENAME="$MATRIX_CODENAME" bash -c '
    set -euo pipefail
    cd build
    build-simple-cdd --profiles myprofile --dist "$MATRIX_CODENAME"
  '
else
  (
    cd build
    build-simple-cdd --profiles myprofile --dist "$MATRIX_CODENAME"
  )
fi

test -d build/images
ls -1 build/images/*.iso > build/iso-list.txt
