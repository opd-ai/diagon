#!/usr/bin/env bash
# Downloads a pinned actionlint release and lints all workflow files.
# Replaces the non-existent `rhysd/actionlint@v1` action reference, which does
# not publish a GitHub Action and caused every workflow run to fail while
# resolving actions.
set -euo pipefail

ACTIONLINT_VERSION="${ACTIONLINT_VERSION:-1.7.7}"
download_script="https://raw.githubusercontent.com/rhysd/actionlint/v${ACTIONLINT_VERSION}/scripts/download-actionlint.bash"

# Download the pinned actionlint binary into the current directory.
bash <(curl -sSfL "$download_script") "$ACTIONLINT_VERSION"

./actionlint -color
