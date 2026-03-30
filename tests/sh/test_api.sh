#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

PORT="${PORT:-61001}"
LOG_FILE="${LOG_DIR}/api.log"
BODY_FILE="${LOG_DIR}/api-body.json"
CSV_INPUT="${CSV_INPUT:-tests/data/csv/input/sample.csv}"
CSV_OUTPUT="${CSV_OUTPUT:-tests/data/csv/output/api_sample_out.csv}"

trap stop_server EXIT

section "API Test"
require_binary
cleanup_outputs

start_server "$PORT" "$LOG_FILE"

section "Reject Invalid Submit"
code="$(http_status POST "http://localhost:${PORT}/jobs" "$BODY_FILE" -H "Content-Type: application/json" -d '{"payload":{}}')"
[[ "$code" == "400" ]] || fail "invalid submit returned $code, want 400"
assert_contains "$(cat "$BODY_FILE")" "Field 'type' is required" "invalid submit missing validation message"
pass "invalid submit returned 400 with validation message"

section "Accept Valid Submit"
payload="$(cat <<JSON
{
  "type": "csv_transform",
  "payload": {
    "input_path": "${CSV_INPUT}",
    "output_path": "${CSV_OUTPUT}"
  }
}
JSON
)"
code="$(http_status POST "http://localhost:${PORT}/jobs" "$BODY_FILE" -H "Content-Type: application/json" -d "$payload")"
[[ "$code" == "202" ]] || fail "valid submit returned $code, want 202"
job_id="$(extract_id < "$BODY_FILE")"
[[ -n "$job_id" ]] || fail "could not extract job id from submit response"
pass "valid submit returned 202 and job id ${job_id}"

section "Status Endpoint"
status_json="$(poll_job "$PORT" "$job_id" 120)"
assert_contains "$status_json" "\"status\":\"completed\"" "job did not complete successfully"
pass "status endpoint reported completed job ${job_id}"

section "List Endpoint"
code="$(http_status GET "http://localhost:${PORT}/jobs" "$BODY_FILE")"
[[ "$code" == "200" ]] || fail "list endpoint returned $code"
assert_contains "$(cat "$BODY_FILE")" "$job_id" "list endpoint missing submitted job"
pass "list endpoint included job ${job_id}"

section "Unknown Job Status"
code="$(http_status GET "http://localhost:${PORT}/jobs/does-not-exist" "$BODY_FILE")"
[[ "$code" == "404" ]] || fail "unknown status endpoint returned $code, want 404"
pass "unknown job status returned 404"

section "Done"
info "API test passed"
