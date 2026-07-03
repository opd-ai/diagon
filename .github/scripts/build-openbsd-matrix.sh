#!/usr/bin/env bash
# Builds the OpenBSD CI job matrix from the OpenBSD matrix file and writes it to
# the GitHub Actions step output. Mirrors build-ci-matrix.sh but for the
# separate OpenBSD pipeline.
set -euo pipefail

ci_matrix=$(jq -c '{include: [.environments[] | {
  environment: .environment,
  openbsd_version: .openbsd_version,
  openbsd_arch: .openbsd_arch,
  openbsd_mirror: .openbsd_mirror,
  go_version: .go_version,
  package_dependencies: .package_dependencies,
  components: .components,
  diagon_repo: .components.diagon.repo,
  diagon_version: .components.diagon.version,
  diagon_build_input: .components.diagon.build_input,
  store_repo: .components.store.repo,
  store_version: .components.store.version,
  store_build_input: .components.store.build_input,
  paywall_repo: .components.paywall.repo,
  paywall_version: .components.paywall.version,
  paywall_build_input: .components.paywall.build_input,
  i2pd_repo: .components.i2pd.repo,
  i2pd_version: .components.i2pd.version,
  i2pd_build_input: .components.i2pd.build_input,
  service_contract_primary: .contract_fixtures.primary,
  service_contract_fixtures: .contract_fixtures.service_contracts
}]}' .github/openbsd-matrix.json)
echo "ci_matrix=$ci_matrix" >> "$GITHUB_OUTPUT"
