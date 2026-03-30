#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

TEMP_JOB="temp_batch_probe"
JOB_FILE="internal/jobs/${TEMP_JOB}.go"
REGISTRY_FILE="internal/registry/payloadConstructor.go"

section "Batch Register Test"
require_binary

assert_file_missing "$JOB_FILE"

register_output="$(EDITOR=true "$BIN" register batch "$TEMP_JOB" 2>&1)"
assert_contains "$register_output" "Registered ${TEMP_JOB}" "register batch did not report success"
assert_file_exists "$JOB_FILE"

job_source="$(cat "$JOB_FILE")"
assert_contains "$job_source" "Scan() ([]Item, error)" "batch template missing Scan"
assert_contains "$job_source" "RunBatch(ctx context.Context, batch []Item)" "batch template missing RunBatch"
assert_contains "$job_source" "Aggregate(results []any)" "batch template missing Aggregate"
assert_contains "$(cat "$REGISTRY_FILE")" "${TEMP_JOB}" "registry missing batch registration"
pass "batch register created scaffold with batch methods"

go build -o Machina ./cmd
pass "batch generated code builds successfully"

unregister_output="$("$BIN" unregister "$TEMP_JOB" 2>&1)"
assert_contains "$unregister_output" "Unregistered ${TEMP_JOB}" "unregister batch did not report success"
assert_file_missing "$JOB_FILE"
pass "batch unregister removed generated artifacts"

section "Done"
info "Batch register test passed"
