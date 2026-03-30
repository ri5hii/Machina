#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

PORT="${PORT:-61000}"
LOG_FILE="${LOG_DIR}/server.log"
BODY_FILE="${LOG_DIR}/server-body.json"

trap stop_server EXIT

section "Server Test"
require_binary
cleanup_outputs

start_server "$PORT" "$LOG_FILE" --workers 2 --queue-size 10

section "Health Endpoint"
show_command "$BIN" health --port "$PORT"
body="$("$BIN" health --port "$PORT" 2>&1)"
assert_contains "$body" "\"serverStatus\"" "health response missing serverStatus"
assert_contains "$body" "\"engineStatus\"" "health response missing engineStatus"
pass "health command returned server and engine status"

section "Shutdown Command"
show_command "$BIN" shutdown --port "$PORT"
shutdown_output="$("$BIN" shutdown --port "$PORT" 2>&1)"
assert_contains "$shutdown_output" "Shutdown signal sent successfully" "shutdown command missing success message"
sleep 1
if curl -sSf "http://localhost:${PORT}/health" >/dev/null 2>&1; then
  fail "server still responded after shutdown"
fi
pass "shutdown command stopped the server"

section "Done"
info "Server test passed"
