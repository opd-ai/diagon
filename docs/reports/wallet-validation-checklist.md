# Diagon Production Wallet Validation Checklist

This checklist validates Monero wallet RPC readiness for production deployments after CI has passed in stubbed mode.

- Environment: debian-12
- Debian: 12 (bookworm)
- Component versions: diagon=main, store=main, paywall=main, i2pd=2.54.0

## CI Baseline

- [ ] Confirm CI Stage 6 smoke output reports `wallet_mode: stubbed`.
- [ ] Confirm production deployment replaces stubbed wallet mode with a real wallet RPC target.

## Secrets And Endpoint Preflight

- Wallet RPC URL: `http://127.0.0.1:18089/json_rpc`
- Paywall health endpoint: `http://127.0.0.1:8081/healthz`
- Paywall settlement endpoint path: `/pay`
- [ ] Verify secret `PAYWALL_WALLET_RPC_PASSWORD` is present via env -> `PAYWALL_WALLET_RPC_PASSWORD`
- [ ] Verify secret `PAYWALL_WALLET_RPC_USER` is present via env -> `PAYWALL_WALLET_RPC_USER`

## Wallet RPC Validation Commands

```bash
curl -fsS -X POST http://127.0.0.1:18089/json_rpc \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":"diagon-wallet-check","method":"get_version"}'

curl -fsS http://127.0.0.1:8081/healthz

curl -fsS -X POST http://127.0.0.1:8081/pay \
  -H 'Content-Type: application/json' \
  --data '{"amount":1,"currency":"XMR"}'
```

## Success Criteria

- [ ] Wallet RPC responds to `get_version` without timeout or auth errors.
- [ ] Paywall health endpoint returns `2xx` after production wallet settings are applied.
- [ ] Paywall settlement path accepts a test request and returns a successful response.
- [ ] Operator captures command output and rollback steps in deployment records.
