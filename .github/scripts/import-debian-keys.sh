#!/usr/bin/env bash
# Imports Debian archive signing keys on a best-effort basis so ISO builds can
# verify package signatures.
set -uo pipefail

gpg --keyserver keyserver.ubuntu.com --recv-keys B8E5F13176D2A7A75220028078DBA3BC47EF2265 || true
gpg --keyserver keyserver.ubuntu.com --recv-keys B8B80B5B623EAB6AD8775C45B7C5D7D6350947F8 || true
gpg --export B8E5F13176D2A7A75220028078DBA3BC47EF2265 B8B80B5B623EAB6AD8775C45B7C5D7D6350947F8 | tee -a /usr/share/keyrings/debian-archive-keyring.gpg > /dev/null || true
