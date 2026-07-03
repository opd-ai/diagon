# Diagon Operator Runbook

## Scope

This runbook covers local single-host operation of Diagon, Store, Paywall, and i2pd on Debian.

- Environment: debian-12
- Debian: 12 (bookworm)
- Component versions: diagon=main, store=main, paywall=main, i2pd=2.54.0

## Config And Secrets

- i2pd config: /etc/diagon/i2pd/i2pd.conf
- paywall config: /etc/diagon/paywall/config.yaml
- store config: /etc/diagon/store/config.yaml
- Secret PAYWALL_WALLET_RPC_USER: env -> PAYWALL_WALLET_RPC_USER (required)
- Secret PAYWALL_WALLET_RPC_PASSWORD: env -> PAYWALL_WALLET_RPC_PASSWORD (required)
- Secret STORE_SESSION_SECRET: file -> /run/secrets/store-session-secret (required)

## Start

```bash
sudo systemctl start diagon-i2pd.service
sudo systemctl start diagon-paywall.service
sudo systemctl start diagon-store.service
```

## Stop

```bash
sudo systemctl stop diagon-store.service
sudo systemctl stop diagon-paywall.service
sudo systemctl stop diagon-i2pd.service
```

## Status

```bash
systemctl status diagon-i2pd.service --no-pager
systemctl status diagon-paywall.service --no-pager
systemctl status diagon-store.service --no-pager
curl -fsS http://127.0.0.1:7070/health
curl -fsS http://127.0.0.1:8081/healthz
curl -fsS http://127.0.0.1:8080/healthz
```

## Logs

```bash
journalctl -u diagon-i2pd.service -n 200 --no-pager
journalctl -u diagon-paywall.service -n 200 --no-pager
journalctl -u diagon-store.service -n 200 --no-pager
```

## Smoke Validation

```bash
curl -fsS -X POST http://127.0.0.1:8081/pay
curl -fsS -X POST http://127.0.0.1:18080/checkout
```

## Recovery

1. If a single service is unhealthy, inspect its unit status and logs, then restart that unit.
2. If dependency order is suspect, stop services in reverse order and start them again in startup order.
3. Re-run the health checks and smoke validation commands before returning the host to service.
4. Preserve operator-managed state under /etc/diagon, /var/lib/diagon, and /var/log/diagon during rollback.

## Package-Owned Units

- diagon-i2pd.service: Diagon i2pd service
- diagon-paywall.service: Diagon paywall service
- diagon-store.service: Diagon store service
