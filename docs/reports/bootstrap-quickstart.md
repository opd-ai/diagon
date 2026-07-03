# Diagon Single-Host Bootstrap Quickstart

This guide provides one deterministic bootstrap profile and one secrets contract for local Debian bring-up.

- Environment: debian-12
- Debian: 12 (bookworm)

## Canonical Inputs

- Profile directory: `profiles`
- Profile name: `myprofile`
- Bootstrap profile: `profiles/local-single-host-bootstrap.json`
- Service contract: `service-contract.json`

## Required Secrets

- `PAYWALL_WALLET_RPC_PASSWORD` (env) required
  - `export PAYWALL_WALLET_RPC_PASSWORD='<redacted>'`
- `PAYWALL_WALLET_RPC_USER` (env) required
  - `export PAYWALL_WALLET_RPC_USER='<redacted>'`
- `STORE_SESSION_SECRET` (file) required
  - `install -m 600 /dev/null /run/secrets/store-session-secret` then write the secret value

## Deterministic Validation

```bash
go run ./cmd/diagonctl \
  --profile-dir profiles \
  --profile-name myprofile \
  --policy-file profiles/validation-policy.json \
  --bootstrap-profile-file profiles/local-single-host-bootstrap.json \
  --service-contract-file service-contract.json \
  --format json
```

## Readiness Probe

```bash
go run ./cmd/diagonctl \
  --profile-dir profiles \
  --profile-name myprofile \
  --policy-file profiles/validation-policy.json \
  --bootstrap-profile-file profiles/local-single-host-bootstrap.json \
  --service-contract-file service-contract.json \
  --probe-live \
  --probe-timeout 45s \
  --probe-interval 250ms \
  --format json
```

## Operator Runbook

```bash
go run ./cmd/diagonctl \
  --profile-dir profiles \
  --profile-name myprofile \
  --policy-file profiles/validation-policy.json \
  --bootstrap-profile-file profiles/local-single-host-bootstrap.json \
  --service-contract-file service-contract.json \
  --integration-matrix-file .github/integration-matrix.json \
  --integration-environment debian-12 \
  --emit-operator-runbook-file ./operator-runbook.md \
  --format json
```

## Success Criteria

- Validation command returns `status=ok`.
- Live probe output returns `aggregated_health.ready=true`.
- Operator runbook is generated without manual profile edits beyond secrets.
