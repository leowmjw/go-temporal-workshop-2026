#!/usr/bin/env bash
set -euo pipefail

ENDPOINT="workshop-user-directory"
TARGET_NS="team-b"
TARGET_TQ="workshop-team-b"

if ! temporal operator namespace describe default >/dev/null 2>&1; then
  echo "Temporal dev server is not reachable at ${TEMPORAL_ADDRESS:-127.0.0.1:7233}."
  echo "Start it first: mise run dev (or mise run temporal)."
  exit 1
fi

for ns in team-a team-b; do
  if temporal operator namespace describe "$ns" >/dev/null 2>&1; then
    echo "namespace $ns: already exists"
  else
    temporal operator namespace create --namespace "$ns" >/dev/null
    echo "namespace $ns: created"
  fi
done

if temporal operator nexus endpoint list 2>/dev/null | grep -q "$ENDPOINT"; then
  echo "endpoint $ENDPOINT: already exists"
else
  # The dev server validates the target namespace through a registry that lags
  # briefly behind namespace creation, so retry a few times.
  for attempt in 1 2 3 4 5; do
    if temporal operator nexus endpoint create \
      --name "$ENDPOINT" \
      --target-namespace "$TARGET_NS" \
      --target-task-queue "$TARGET_TQ" \
      --description "TeamB user-directory service (workshop Nexus demo)" >/dev/null 2>&1; then
      echo "endpoint $ENDPOINT: created -> $TARGET_NS/$TARGET_TQ"
      break
    fi
    if [ "$attempt" -eq 5 ]; then
      echo "endpoint $ENDPOINT: failed to create after $attempt attempts" >&2
      exit 1
    fi
    sleep 1.5
  done
fi

echo
if [ "${SETUP_NEXUS_NO_HINT:-}" = "1" ]; then
  exit 0
fi

echo "Wiring complete. If the worker was already running, restart it so it joins team-a/team-b:"
echo "  overmind restart worker    # or stop and re-run: mise run dev"
