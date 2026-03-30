#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

section "Status Validation Test"
require_binary

output="$("$BIN" status 2>&1 || true)"
assert_contains "$output" "missing job id" "status without id missing validation"
pass "status rejects missing job id"

output="$("$BIN" status abc --watch --interval 0 2>&1 || true)"
assert_contains "$output" "interval must be greater than 0" "status invalid interval missing validation"
pass "status rejects zero interval"

output="$("$BIN" status abc --nope 2>&1 || true)"
assert_contains "$output" "invalid status flag" "status unknown flag missing validation"
pass "status rejects unknown flags"

output="$("$BIN" status abc --port nope 2>&1 || true)"
assert_contains "$output" "invalid port: nope" "status invalid port missing validation"
pass "status rejects non-numeric port"

section "Done"
info "Status validation test passed"
