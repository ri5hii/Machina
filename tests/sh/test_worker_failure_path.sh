#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

PORT="${PORT:-61005}"
LOG_FILE="${LOG_DIR}/worker-failure.log"
BODY_FILE="${LOG_DIR}/worker-failure-body.json"

trap stop_server EXIT

section "Worker Failure Path Test"
require_binary
cleanup_outputs

start_server "$PORT" "$LOG_FILE"

payload='{"type":"csv_transform","payload":{"input_path":"tests/data/csv/input/does-not-exist.csv","output_path":"tests/data/csv/output/worker_failure.csv"}}'
code="$(http_status POST "http://localhost:${PORT}/jobs" "$BODY_FILE" -H "Content-Type: application/json" -d "$payload")"
[[ "$code" == "202" ]] || fail "expected failed-path job submit to be accepted first, got $code"
job_id="$(extract_id < "$BODY_FILE")"
[[ -n "$job_id" ]] || fail "could not extract failed-path job id"
pass "failure-path submit returned accepted job ${job_id}"

status_json="$(poll_job "$PORT" "$job_id" 120)"
assert_contains "$status_json" "\"status\":\"failed\"" "failed-path job did not fail"
assert_contains "$status_json" "\"error\"" "failed-path job missing error details"
pass "worker failure path recorded failed status and error"

section "Done"
info "Worker failure path test passed"
