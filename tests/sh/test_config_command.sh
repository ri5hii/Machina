#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

trap restore_config EXIT

section "Config Command Test"
require_binary
backup_config

before="$("$BIN" config 2>&1)"
assert_contains "$before" "Port:" "config output missing port"
assert_contains "$before" "Worker count:" "config output missing worker count"
pass "config prints current settings"

output="$("$BIN" config --port 61010 --workers 4 --queue-size 10 2>&1)"
assert_contains "$output" "port set to: 61010" "config did not update port"
assert_contains "$output" "worker count set to: 4" "config did not update worker count"
assert_contains "$output" "queue size set to: 10" "config did not update queue size"
pass "config updates persisted values"

config_file="$(cat "$CONFIG_FILE")"
assert_contains "$config_file" "\"port\": 61010" "config file missing updated port"
assert_contains "$config_file" "\"workerCount\": 4" "config file missing updated workerCount"
assert_contains "$config_file" "\"queuesize\": 10" "config file missing updated queue size"
pass "config.json contains updated values"

invalid="$("$BIN" config --workers 2 2>&1)"
assert_contains "$invalid" "must be between 4 and 10" "config invalid workerCount missing validation"
pass "config rejects invalid worker count"

section "Done"
info "Config command test passed"
