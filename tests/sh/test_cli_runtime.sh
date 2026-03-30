#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

PORT="${PORT:-61003}"
LOG_FILE="${LOG_DIR}/cli-runtime.log"
CSV_INPUT="${CSV_INPUT:-tests/data/csv/input/sample.csv}"
CSV_OUTPUT="${CSV_OUTPUT:-tests/data/csv/output/cli_runtime_out.csv}"

trap stop_server EXIT

section "CLI Runtime Test"
require_binary
cleanup_outputs

start_server "$PORT" "$LOG_FILE"

section "Submit"
show_command "$BIN" submit csv-transform "$CSV_INPUT" "$CSV_OUTPUT" --port "$PORT"
submit_output="$("$BIN" submit csv-transform "$CSV_INPUT" "$CSV_OUTPUT" --port "$PORT" 2>&1)"
job_id="$(printf '%s' "$submit_output" | extract_id)"
[[ -n "$job_id" ]] || fail "submit command did not return a job id"
pass "submit command returned job id ${job_id}"

section "Status"
status_output="$("$BIN" status "$job_id" --port "$PORT" 2>&1)"
assert_contains "$status_output" "\"id\"" "status command output missing job id"
pass "status command returned job payload for ${job_id}"

section "List"
list_output="$("$BIN" list --port "$PORT" 2>&1)"
assert_contains "$list_output" "$job_id" "list command output missing submitted job id"
pass "list command returned submitted job ${job_id}"

section "Watch"
watch_output="$("$BIN" status "$job_id" --watch --interval 1 --port "$PORT" 2>&1)"
assert_contains "$watch_output" "\"status\"" "watch command output missing status"
assert_contains "$watch_output" "\"completed\"" "watch command did not stop on completed"
pass "watch command stopped on completed status"

section "Done"
info "CLI runtime test passed"
