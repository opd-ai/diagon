#!/usr/bin/env bash
# Builds the Go stub service and launches i2pd/store/paywall stubs plus their
# tunnel listeners for integration bootstrap probing.
set -euo pipefail

mkdir -p artifacts .tmp
go build -o .tmp/diagon-stub-service ./cmd/diagon-stub-service

start_stub() {
  local port="$1"
  local name="$2"

  nohup .tmp/diagon-stub-service --port "$port" > "artifacts/${name}.log" 2>&1 &
  echo $! > ".tmp/${name}.pid"
  sleep 1
}

start_stub 7070 i2pd
start_stub 8081 paywall
start_stub 8080 store
start_stub 18080 store-tunnel
start_stub 18081 paywall-tunnel
