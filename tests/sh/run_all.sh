#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

SCRIPTS=(
  "tests/sh/test_server.sh"
  "tests/sh/test_api.sh"
  "tests/sh/test_engine.sh"
  "tests/sh/test_profile_integrity.sh"
  "tests/sh/test_start_flags.sh"
  "tests/sh/test_status_validation.sh"
  "tests/sh/test_submit_validation.sh"
  "tests/sh/test_config_command.sh"
  "tests/sh/test_worker_failure_path.sh"
  "tests/sh/test_queue_recovery.sh"
  "tests/sh/test_cli_runtime.sh"
  "tests/sh/test_cli_registry.sh"
  "tests/sh/test_register_batch.sh"
)

echo "== Build Binary =="
go build -o Machina ./cmd

for script in "${SCRIPTS[@]}"; do
  echo
  echo "== Running ${script} =="
  bash "$script"
done

echo
echo "All shell tests passed."
