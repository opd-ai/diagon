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
- i2pd tunnel contract entries including tunnel type and local listener-to-service target mappings
- Endpoint compatibility checks for Store -> Paywall API links
- Optional live readiness probes for health endpoints, listeners, and dependency sequencing

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

### Run live service probes (CI bootstrap handoff)

```bash
go run ./cmd/diagonctl \
	--profile-dir profiles \
	--profile-name myprofile \
	--policy-file profiles/validation-policy.json \
	--service-contract-file profiles/service-contract.json \
	--probe-live \
	--probe-timeout 45s \
	--probe-interval 750ms \
	--format json
```

The service contract validator enforces:

- Required service definitions for `i2pd`, `store`, and `paywall`
- Local-only listener and health endpoint hosts (`localhost`, `127.0.0.1`, `::1`)
- Startup ordering (`i2pd` must start before `store` and `paywall`)
- Dependency integrity and cycle detection
- API link endpoint port compatibility (`store` to `paywall`)
- i2pd tunnel type validation (`client`, `http`, `http-proxy`, `socks`, `server`)
- Local-only tunnel listener/target addresses and target-service port compatibility
- Required tunnel mappings for both `store` and `paywall`

When `--probe-live` is enabled, `diagonctl` also performs runtime checks:

- TCP listener reachability for each service `listen` address
- HTTP readiness checks for each service `health_url` (expects `2xx`)
- Dependency sequencing validation (`depends_on` services must become ready first)
- Startup-order signal warnings when higher-order services become ready before lower-order services

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

- Stage 1: static checks (`actionlint`, `gofmt`, `go vet`)
- Stage 2: build artifacts and build-metadata emission for Diagon/Store/Paywall/i2pd pinned matrix entries
- Stage 3: unit tests (`go test ./...`)
- Stage 4: integration bootstrap and live readiness probes (`--probe-live`)
- Stage 5: contract tests for profile + matrix-defined service-contract fixture compatibility
- Stage 6: end-to-end smoke transaction harness with stubbed wallet mode
- Stage 7: Debian packaging verification (`simple-cdd`, ISO build output checks)
- Stage 8: checksum + version manifest bundle, with release asset publishing on release events

Quality gates are modeled as explicit jobs:

- `Quality Gate - Merge (Stages 1-7)` passes only if Stages 1 through 7 succeed
- `Quality Gate - Release (Stage 8)` passes only if Stage 8 succeeds on release events

These gates are designed to be used as required-status checks in branch and release protection rules.