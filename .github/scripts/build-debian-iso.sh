#!/usr/bin/env bash
# Builds the Debian installer ISO with simple-cdd and verifies the output.
# Requires MATRIX_CODENAME in the environment.
set -euo pipefail

mkdir -p build
cp -r profiles build/
cd build
build-simple-cdd --profiles myprofile --dist "$MATRIX_CODENAME"

test -d images
ls -1 images/*.iso > iso-list.txt
