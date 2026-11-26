#!/usr/bin/env bash
# Starts the full workshop stack with Nexus namespaces in place.
#
# Ordering matters: the worker checks for the team-a/team-b namespaces once, at
# boot, and permanently skips the Nexus team workers if they are missing. So the
# Temporal server must be up and the namespaces must exist before the worker
# starts. Overmind does not order processes, so this script sequences it.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
mkdir -p .data

ADDRESS="${TEMPORAL_ADDRESS:-127.0.0.1:7233}"
HOSTPORT="${ADDRESS#*://}"
HOST="${HOSTPORT%%:*}"
PORT="${HOSTPORT##*:}"

# 0.0.0.0 is a bind address, not a connect address.
case "$HOST" in
  0.0.0.0 | "") HOST=127.0.0.1 ;;
esac

DATA_DIR="${WORKSHOP_DATA_DIR:-$ROOT/.data}"
mkdir -p "$DATA_DIR"
LOG="$DATA_DIR/temporal.log"

started_temporal=""

cleanup() {
  if [ -n "$started_temporal" ]; then
    echo "stopping Temporal (pid $started_temporal)"
    # mise runs the real server as a child of its wrapper, so stop the children
    # first, then the wrapper. Ignore errors: a SIGINT may have killed it already.
    pkill -TERM -P "$started_temporal" 2>/dev/null || true
    kill -TERM "$started_temporal" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

temporal_ready() {
  temporal --address "$HOST:$PORT" operator namespace describe default >/dev/null 2>&1
}

if temporal_ready; then
  echo "Temporal already running at $HOST:$PORT"
else
  echo "starting Temporal at $HOST:$PORT (log: $LOG)"
  mise run temporal >"$LOG" 2>&1 &
  started_temporal=$!

  for _ in $(seq 1 120); do
    if temporal_ready; then
      break
    fi
    if ! kill -0 "$started_temporal" 2>/dev/null; then
      echo "Temporal exited during startup. Last lines of $LOG:" >&2
      tail -20 "$LOG" >&2 || true
      exit 1
    fi
    sleep 0.5
  done

  if ! temporal_ready; then
    echo "Temporal did not become ready at $HOST:$PORT. See $LOG" >&2
    exit 1
  fi
  echo "Temporal ready"
fi

# Idempotent: creates team-a/team-b and the Nexus endpoint only if missing.
echo "wiring Nexus namespaces"
TEMPORAL_ADDRESS="$HOST:$PORT" SETUP_NEXUS_NO_HINT=1 bash scripts/setup-nexus.sh

echo "starting worker and server"
overmind start -f Procfile
