#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

PORT="${PORT:-61004}"
LOG_FILE="${LOG_DIR}/start-flags.log"
BODY_FILE="${LOG_DIR}/start-flags-body.json"
ENC_INPUT_DIR="${ENC_INPUT_DIR:-tests/data/encrypt/input}"

trap stop_server EXIT

section "Start Flags Test"
require_binary
cleanup_outputs

start_server "$PORT" "$LOG_FILE" --workers 2 --queue-size 1

log_content="$(cat "$LOG_FILE")"
assert_contains "$log_content" "\"Worker count\":2" "start log missing worker count override"
assert_contains "$log_content" "\"Port\":\":${PORT}\"" "start log missing port override"
pass "start flags applied worker count and port overrides"

output_dir="tests/data/encrypt/output/start_flags_probe"
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

accepted=0
rejected=0
for _ in 1 2 3 4; do
  code="$(http_status POST "http://localhost:${PORT}/jobs" "$BODY_FILE" -H "Content-Type: application/json" -d "$payload")"
  if [[ "$code" == "202" ]]; then
    accepted=$((accepted + 1))
  elif [[ "$code" == "503" ]]; then
    rejected=$((rejected + 1))
  fi
done

[[ "$accepted" -ge 1 ]] || fail "expected at least one accepted submit with start flags"
[[ "$rejected" -ge 1 ]] || fail "expected queue-size override to reject at least one submit"
pass "queue-size override affected runtime queue behavior"

section "Done"
info "Start flags test passed"
