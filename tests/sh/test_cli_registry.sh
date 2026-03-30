#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

TEMP_JOB="temp_registry_probe"
JOB_FILE="internal/jobs/${TEMP_JOB}.go"
REGISTRY_FILE="internal/registry/payloadConstructor.go"

section "CLI Registry Test"
require_binary

if [[ -e "$JOB_FILE" ]]; then
  fail "temporary job file already exists: $JOB_FILE"
fi

section "Types Before Register"
types_before="$("$BIN" types 2>&1)"
if [[ "$types_before" == *"$TEMP_JOB"* ]]; then
  fail "temporary job already present before register"
fi
pass "types output does not contain ${TEMP_JOB} before registration"

section "Register"
show_command EDITOR=true "$BIN" register singleRun "$TEMP_JOB"
register_output="$(EDITOR=true "$BIN" register singleRun "$TEMP_JOB" 2>&1)"
assert_contains "$register_output" "Registered ${TEMP_JOB}" "register command did not report success"
assert_file_exists "$JOB_FILE"
assert_contains "$(cat "$REGISTRY_FILE")" "${TEMP_JOB}" "registry file missing registered job"
pass "register created source file and updated registry"

section "Types After Register"
types_after="$("$BIN" types 2>&1)"
assert_contains "$types_after" "$TEMP_JOB" "types command missing registered temp job"
pass "types output includes ${TEMP_JOB} after registration"

section "Generated Code Builds"
show_command go build -o Machina ./cmd
go build -o Machina ./cmd
pass "generated code builds successfully"

section "Unregister"
unregister_output="$("$BIN" unregister "$TEMP_JOB" 2>&1)"
assert_contains "$unregister_output" "Unregistered ${TEMP_JOB}" "unregister command did not report success"
assert_file_missing "$JOB_FILE"
pass "unregister removed source file and registry entry"

section "Types After Unregister"
types_final="$("$BIN" types 2>&1)"
if [[ "$types_final" == *"$TEMP_JOB"* ]]; then
  fail "types command still contains temp job after unregister"
fi
pass "types output no longer includes ${TEMP_JOB}"

section "Done"
info "CLI registry test passed"
