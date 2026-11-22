#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
mkdir -p .data

TEMPORAL_LOG=.data/temporal.log
WORKER_LOG=.data/worker.log
SERVER_LOG=.data/server.log

temporal server start-dev --namespace default --db-filename "$ROOT/.data/e2e.db" >"$TEMPORAL_LOG" 2>&1 &
TEMPORAL_PID=$!
trap 'kill ${TEMPORAL_PID:-} ${WORKER_PID:-} ${SERVER_PID:-} 2>/dev/null || true' EXIT

for _ in $(seq 1 60); do
  temporal operator namespace describe default >/dev/null 2>&1 && break
  sleep 0.25
done

TEMPORAL_ADDRESS=127.0.0.1:7233 go run ./cmd/worker >"$WORKER_LOG" 2>&1 &
WORKER_PID=$!
TEMPORAL_ADDRESS=127.0.0.1:7233 WORKSHOP_HTTP_ADDR=127.0.0.1:8080 go run ./cmd/server >"$SERVER_LOG" 2>&1 &
SERVER_PID=$!

for _ in $(seq 1 40); do
  curl --fail --silent http://127.0.0.1:8080/health >/dev/null 2>&1 && break
  sleep 0.25
done

curl --fail --silent -X POST http://127.0.0.1:8080/api/update
curl --fail --silent http://127.0.0.1:8080/api/snapshot
