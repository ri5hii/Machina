#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

section "Submit Validation Test"
require_binary

output="$("$BIN" submit 2>&1 || true)"
assert_contains "$output" "Usage: machina submit" "submit without args missing help output"
pass "submit shows help when arguments are missing"

output="$("$BIN" submit unknown-job in out 2>&1 || true)"
assert_contains "$output" "unknown job name" "submit unknown job missing validation"
pass "submit rejects unknown job names"

output="$("$BIN" submit csv-transform in out --port nope 2>&1 || true)"
assert_contains "$output" "invalid port: nope" "submit bad port missing numeric validation"
pass "submit rejects non-numeric ports"

output="$("$BIN" submit csv-transform in out --bogus 2>&1 || true)"
assert_contains "$output" "unknown flag" "submit unknown flag missing validation"
pass "submit rejects unknown flags"

section "Done"
info "Submit validation test passed"
