#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

PORT="${PORT:-61002}"
LOG_FILE="${LOG_DIR}/engine.log"
BODY_FILE="${LOG_DIR}/engine-body.json"
ENC_INPUT_DIR="${ENC_INPUT_DIR:-tests/data/encrypt/input}"

trap stop_server EXIT

section "Engine Test"
require_binary
cleanup_outputs

start_server "$PORT" "$LOG_FILE" --workers 1 --queue-size 1

section "Queue Pressure"
accepted=0
rejected=0

for i in 1 2 3 4 5; do
  output_dir="tests/data/encrypt/output/engine_${i}"
  mkdir -p "$output_dir"
  payload="$(cat <<JSON
{
  "type": "file_encrypt",
  "payload": {
    "folder_path": "${ENC_INPUT_DIR}",
    "output_path": "${output_dir}"
  }
}
JSON
)"
  code="$(http_status POST "http://localhost:${PORT}/jobs" "$BODY_FILE" -H "Content-Type: application/json" -d "$payload")"
  if [[ "$code" == "202" ]]; then
    accepted=$((accepted + 1))
  elif [[ "$code" == "503" ]]; then
    rejected=$((rejected + 1))
  else
    fail "unexpected status code from queue pressure submit: $code"
  fi
done

[[ "$accepted" -ge 1 ]] || fail "expected at least one accepted job"
[[ "$rejected" -ge 1 ]] || fail "expected at least one rejected job when queue is full"
pass "queue pressure produced ${accepted} accepted and ${rejected} rejected submissions"

section "Engine Completion"
code="$(http_status GET "http://localhost:${PORT}/jobs" "$BODY_FILE")"
[[ "$code" == "200" ]] || fail "jobs list returned $code"
assert_contains "$(cat "$BODY_FILE")" "\"status\"" "jobs list missing statuses"
pass "jobs list returned tracked engine statuses"

section "Done"
info "Engine test passed"
