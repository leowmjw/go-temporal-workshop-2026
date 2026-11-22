#!/usr/bin/env bash
set -euo pipefail
mkdir -p .data
temporal server start-dev --namespace "${TEMPORAL_NAMESPACE:-default}" --db-filename "${WORKSHOP_DATA_DIR:-.data}/temporal.db"
