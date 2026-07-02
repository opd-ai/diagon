# diagon

`diagon` currently contains Debian installer profile assets and CI automation for building an installer ISO.

## Profile Validation CLI

This repository now includes `diagonctl`, a lightweight validator for Debian profile inputs.

It validates:

- Presence of required packages for the current baseline (`curl`, `openssh-server`, `i2pd`)
- Required preseed keys and safe baseline values (`passwd/root-login=false`, non-empty `time/zone`)
- Warnings for wildcard package usage (for reproducibility)

### Run validation

```bash
go run ./cmd/diagonctl --profile-dir profiles --profile-name myprofile
```

### Run tests

```bash
go test ./...
```