# diagon

`diagon` currently contains Debian installer profile assets and CI automation for building an installer ISO.

## Profile Validation CLI

This repository now includes `diagonctl`, a lightweight validator for Debian profile inputs.

It validates:

- Presence of required packages for the current baseline (`curl`, `openssh-server`, `i2pd`)
- Required preseed keys and safe baseline values (`passwd/root-login=false`, non-empty `time/zone`)
- Warnings for wildcard package usage (for reproducibility)
- Optional JSON policy contracts to define required package/preseed sets per environment
- Duplicate package and duplicate preseed key detection (warning-level)
- Service integration contracts for local i2pd, Store, and Paywall topology
- Local single-host bootstrap profile defaults, startup sequencing, expected tunnels, and secrets sources
- i2pd tunnel contract entries including tunnel type and local listener-to-service target mappings
- Endpoint compatibility checks for Store -> Paywall API links
- Optional live readiness probes for health endpoints, listeners, and dependency sequencing
- Phase 4 release-candidate smoke plans, operator runbooks, and version-frozen release baseline manifests
- Production wallet validation checklist generation for Monero RPC readiness handoff
- Debian fallback compose/service bundle generation with manual install guidance for interim delivery

### Policy file format

`diagonctl` can load a policy contract from JSON:

```json
{
	"required_packages": ["curl", "i2pd", "openssh-server"],
	"required_preseed": {
		"passwd/root-login": "false",
		"time/zone": "*non-empty*"
	}
}
```

`*non-empty*` is a sentinel value meaning the key must exist and have a non-empty value.

### Run validation

```bash
go run ./cmd/diagonctl --profile-dir profiles --profile-name myprofile
```

### Run validation with policy file and JSON output

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--format json
```

### Validate service integration contract

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--service-contract-file profiles/service-contract.json \
	--format json
```

### Validate the Phase 1 single-host bootstrap profile

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--bootstrap-profile-file profiles/local-single-host-bootstrap.json \
	--format json
```

### Run live service probes (CI bootstrap handoff)

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--bootstrap-profile-file profiles/local-single-host-bootstrap.json \
	--service-contract-file profiles/service-contract.json \
	--probe-live \
	--probe-timeout 45s \
	--probe-interval 750ms \
	--format json
```

### Generate injected component config bundle (Phase 2 wiring)

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--bootstrap-profile-file profiles/local-single-host-bootstrap.json \
	--service-contract-file profiles/service-contract.json \
	--emit-config-injection-file /tmp/diagon-config-injection.json \
	--format json
```

Use `--emit-config-injection-file -` to write the generated bundle to stdout.

### Generate Debian package baseline bundle (Phase 3 packaging)

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--bootstrap-profile-file profiles/local-single-host-bootstrap.json \
	--service-contract-file profiles/service-contract.json \
	--emit-debian-package-file /tmp/diagon-debian-package.json \
	--format json
```

Use `--emit-debian-package-file -` to write the generated packaging baseline to stdout.

### Generate Phase 4 release-candidate smoke plan

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--bootstrap-profile-file profiles/local-single-host-bootstrap.json \
	--service-contract-file profiles/service-contract.json \
	--emit-release-smoke-file /tmp/diagon-release-smoke.json \
	--format json
```

Use `--emit-release-smoke-file -` to write the generated smoke plan to stdout.

### Generate operator runbook

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--bootstrap-profile-file profiles/local-single-host-bootstrap.json \
	--service-contract-file profiles/service-contract.json \
	--integration-matrix-file .github/integration-matrix.json \
	--integration-environment debian-12 \
	--emit-operator-runbook-file /tmp/diagon-operator-runbook.md \
	--format json
```

Use `--emit-operator-runbook-file -` to write the generated runbook to stdout.

### Generate single-host bootstrap quickstart guide

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--bootstrap-profile-file profiles/local-single-host-bootstrap.json \
	--service-contract-file profiles/service-contract.json \
	--integration-matrix-file .github/integration-matrix.json \
	--integration-environment debian-12 \
	--emit-bootstrap-quickstart-file /tmp/diagon-bootstrap-quickstart.md \
	--format json
```

Use `--emit-bootstrap-quickstart-file -` to write the generated quickstart guide to stdout.

### Generate production wallet validation checklist

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--bootstrap-profile-file profiles/local-single-host-bootstrap.json \
	--service-contract-file profiles/service-contract.json \
	--integration-matrix-file .github/integration-matrix.json \
	--integration-environment debian-12 \
	--emit-wallet-validation-checklist-file /tmp/diagon-wallet-validation-checklist.md \
	--format json
```

Use `--emit-wallet-validation-checklist-file -` to write the generated checklist to stdout.

### Generate release candidate baseline manifest

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--integration-matrix-file .github/integration-matrix.json \
	--integration-environment debian-12 \
	--emit-release-baseline-file /tmp/diagon-release-baseline.json \
	--format json
```

Use `--emit-release-baseline-file -` to write the generated release baseline to stdout.

### Generate roadmap Definition of Done report

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--bootstrap-profile-file profiles/local-single-host-bootstrap.json \
	--service-contract-file profiles/service-contract.json \
	--integration-matrix-file .github/integration-matrix.json \
	--integration-environment debian-12 \
	--probe-live \
	--probe-timeout 45s \
	--probe-interval 250ms \
	--definition-of-done-release-bundle-dir release-artifacts/manifest \
	--emit-definition-of-done-file /tmp/diagon-definition-of-done.json \
	--format json
```

Use `--emit-definition-of-done-file -` to write the generated definition-of-done report to stdout.

The report tracks Section 7 roadmap criteria as `passed`, `pending`, or `failed`:

- single-profile onboarding + secrets handoff evidence
- live i2pd/service/tunnel readiness evidence
- config-injection + aggregated health evidence
- CI stage quality-gate evidence from the integration baseline
- release artifact evidence (`SHA256SUMS`, `version-manifest.json`, `operator-runbook.md`)

### Generate Debian dependency manifest (risk mitigation)

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--integration-matrix-file .github/integration-matrix.json \
	--integration-environment debian-12 \
	--emit-debian-dependency-manifest-file /tmp/diagon-debian-dependencies.json \
	--format json
```

Use `--emit-debian-dependency-manifest-file -` to write the generated dependency manifest to stdout.

### Generate Debian fallback compose/service bundle

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--bootstrap-profile-file profiles/local-single-host-bootstrap.json \
	--service-contract-file profiles/service-contract.json \
	--integration-matrix-file .github/integration-matrix.json \
	--integration-environment debian-12 \
	--emit-debian-compose-bundle-file /tmp/diagon-compose-bundle.json \
	--format json
```

Use `--emit-debian-compose-bundle-file -` to write the generated fallback bundle to stdout.

The Debian package baseline generator defines:

- Package layout expectations for binaries, configs, logs, state, and runtime directories
- Systemd unit definitions for `diagon-i2pd.service`, `diagon-paywall.service`, and `diagon-store.service`
- Startup dependencies derived from the service contract (`i2pd` before `paywall`, `paywall` before `store`)
- Post-install validation checks for unit enablement, config-file presence, and local health endpoints
- Uninstall and rollback expectations that stop services in reverse order, preserve `/etc/diagon`, `/var/lib/diagon`, and `/var/log/diagon`, and remove only `/run/diagon`

The Phase 4 release-candidate generators define:

- A reusable smoke plan for service boot, marketplace access through the Store i2pd tunnel, direct Paywall settlement checks, and post-restart validation
- An operator runbook with start, stop, status, log, smoke-validation, and recovery commands derived from the frozen bootstrap and service contract inputs
- A release baseline manifest that freezes Debian/component versions, contract fixtures, and the recommended integration-baseline tag from `.github/integration-matrix.json`
- A production wallet validation checklist that captures secrets preflight, Monero wallet RPC probes, and paywall settlement verification commands

The Debian fallback compose/service bundle generator defines:

- A pinned-image `compose.yaml` for `i2pd`, `paywall`, and `store` with dependency-aware startup ordering
- A `diagon-compose.service` systemd unit for host-level lifecycle control of the compose stack
- A secrets-aware environment template and a manual install guide for Debian operators
- CI validation checks for compose model generation, service health endpoints, and expected i2pd tunnel listener reachability

The service contract validator enforces:

- Required service definitions for `i2pd`, `store`, and `paywall`
- Local-only listener and health endpoint hosts (`localhost`, `127.0.0.1`, `::1`)
- Startup ordering (`i2pd` must start before `store` and `paywall`)
- Dependency integrity and cycle detection
- API link endpoint port compatibility (`store` to `paywall`)
- i2pd tunnel type validation (`client`, `http`, `http-proxy`, `socks`, `server`)
- Local-only tunnel listener/target addresses and target-service port compatibility
- Required tunnel mappings for both `store` and `paywall`

The bootstrap profile validator enforces:

- Deterministic Phase 1 startup order: `i2pd`, `paywall`, `store`, `diagonctl`
- Local-only bind and health endpoints for the bootstrap components
- Default local Store -> Paywall endpoint wiring and stubbed Paywall wallet mode
- Absolute Debian-oriented config and secret file paths where file-backed secrets are used
- Expected i2pd tunnel names for the single-host bootstrap profile

When `--probe-live` is enabled, `diagonctl` also performs runtime checks:

- TCP listener reachability for each service `listen` address
- HTTP readiness checks for each service `health_url` (expects `2xx`)
- Capped retry backoff between readiness attempts so transient startup races do not require a fixed high-frequency poll rate
- TCP listener reachability for each expected i2pd tunnel listener
- Dependency sequencing validation (`depends_on` services must become ready first)
- Startup-order signal warnings when higher-order services become ready before lower-order services

Runtime probe output now includes an aggregated component health view (`aggregated_health`) in JSON mode. This aggregation is marked failed when any component readiness probe fails.

### Strict mode (warnings become errors)

```bash
go run ./cmd/diagonctl --profile-dir profiles --profile-name myprofile --strict
```

### Run tests

```bash
go test ./...
```

## CI Stage Pipeline

The GitHub Actions workflow is now split into explicit Stage 1 through Stage 8 jobs with environment metadata for Debian 12 (`bookworm`):

Integration versions and contract fixtures are sourced from [.github/integration-matrix.json](.github/integration-matrix.json). This matrix pins a Debian baseline (`debian_version` + `debian_codename`), freezes packaging dependencies (`package_dependencies`), pins upstream build inputs for Store and Paywall, and defines the service-contract fixtures executed in CI contract testing.

- Stage 1: static checks for pinned matrix components, including pinned-ref resolution for every repo plus `gofmt` and `go vet` for Diagon, Store, and Paywall
- Stage 2: Debian-bookworm artifact builds for Diagon, Store, and Paywall in a pinned `golang:<go_version>-<debian_codename>` container, plus build-metadata emission for the full matrix
- Stage 3: readonly unit tests with lockfile cleanliness enforced after each run. Diagon runs its full first-party suite (`GOFLAGS=-mod=readonly go test ./...`); pinned third-party components (Store, Paywall) run in `-short` mode against their locked dependency state, skipping known non-hermetic upstream network tests so the pipeline stays reproducible and flake-free
- Stage 4: integration bootstrap and live readiness probes (`--probe-live`), plus a timeout regression that proves readiness failures surface when a required service never becomes ready
- Stage 5: contract tests for profile + matrix-defined service-contract fixture compatibility
- Stage 6: generated smoke plan plus end-to-end smoke and graceful-restart validation with stubbed wallet mode
- Stage 6: generated smoke plan plus end-to-end smoke and graceful-restart validation with stubbed wallet mode, including explicit `wallet_mode=stubbed` assertions
- Stage 7: Debian packaging verification (`simple-cdd`, ISO build output checks)
- Stage 7: Debian packaging verification on a pinned Debian container image, with dependency installability verified from an emitted dependency manifest plus ISO build output checks
- Stage 8: checksum + version-frozen release baseline + operator runbook bundle, with release asset publishing on release events
- Stage 8: checksum + version-frozen release baseline + operator runbook + production wallet validation checklist bundle, with release asset publishing on release events

Quality gates are modeled as explicit jobs:

- `Quality Gate - Merge (Stages 1-7)` passes only if Stages 1 through 7 succeed
- `Quality Gate - Release (Stage 8)` passes only if Stage 8 succeeds on release events

These gates are designed to be used as required-status checks in branch and release protection rules.