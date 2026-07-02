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

## CI Integration Probe Stage

The GitHub Actions workflow now includes a Stage 4 bootstrap/probe path that:

- Launches local stub `i2pd`, `paywall`, and `store` HTTP services on contract ports
- Executes `diagonctl` with `--probe-live` against `profiles/service-contract.json`
- Publishes `service-probe.json` and per-service logs as workflow artifacts (`service-probe-report`)

This provides deterministic CI evidence that readiness gates and runtime contract checks execute successfully.