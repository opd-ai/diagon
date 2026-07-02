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

The service contract validator enforces:

- Required service definitions for `i2pd`, `store`, and `paywall`
- Local-only listener and health endpoint hosts (`localhost`, `127.0.0.1`, `::1`)
- Startup ordering (`i2pd` must start before `store` and `paywall`)
- Dependency integrity and cycle detection
- API link endpoint port compatibility (`store` to `paywall`)

### Strict mode (warnings become errors)

```bash
go run ./cmd/diagonctl --profile-dir profiles --profile-name myprofile --strict
```

### Run tests

```bash
go test ./...
```