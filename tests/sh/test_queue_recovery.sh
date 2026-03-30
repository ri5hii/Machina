#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

PORT="${PORT:-61006}"
LOG_FILE="${LOG_DIR}/queue-recovery.log"
BODY_FILE="${LOG_DIR}/queue-recovery-body.json"
ENC_INPUT_DIR="${ENC_INPUT_DIR:-tests/data/encrypt/input}"

trap stop_server EXIT

section "Queue Recovery Test"
require_binary
cleanup_outputs

start_server "$PORT" "$LOG_FILE" --workers 1 --queue-size 1

first_id=""
rejected=0
for i in 1 2 3 4; do
  output_dir="tests/data/encrypt/output/recovery_${i}"
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
    [[ -z "$first_id" ]] && first_id="$(extract_id < "$BODY_FILE")"
  elif [[ "$code" == "503" ]]; then
    rejected=$((rejected + 1))
  fi
done

[[ -n "$first_id" ]] || fail "queue recovery test did not get an accepted job"
[[ "$rejected" -ge 1 ]] || fail "queue recovery test never saturated the queue"
pass "queue saturation produced initial rejection(s)"

poll_job "$PORT" "$first_id" 300 >/dev/null
pass "first queued job eventually completed"

recovery_output="tests/data/encrypt/output/recovery_final"
mkdir -p "$recovery_output"
payload="$(cat <<JSON
{
  "type": "file_encrypt",
  "payload": {
    "folder_path": "${ENC_INPUT_DIR}",
    "output_path": "${recovery_output}"
  }
}
JSON
)"
code="$(http_status POST "http://localhost:${PORT}/jobs" "$BODY_FILE" -H "Content-Type: application/json" -d "$payload")"
[[ "$code" == "202" ]] || fail "queue did not recover; final submit returned $code"
final_id="$(extract_id < "$BODY_FILE")"
[[ -n "$final_id" ]] || fail "queue recovery final job missing id"
pass "queue accepted a new job after earlier pressure"

section "Done"
info "Queue recovery test passed"
