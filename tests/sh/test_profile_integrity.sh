#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

section "Types Integrity Test"
require_binary

source_job_types="$(registered_job_types_from_source | tr '\n' ' ')"
cli_job_types="$("$BIN" types 2>&1 | tr -d '[]",' | xargs)"

for job_type in $(registered_job_types_from_source); do
  [[ "$cli_job_types" == *"$job_type"* ]] || fail "types output missing ${job_type}"
done

source_count="$(registered_job_types_from_source | wc -l | tr -d ' ')"
cli_count="$(printf '%s\n' "$cli_job_types" | xargs -n1 2>/dev/null | sed '/^$/d' | wc -l | tr -d ' ')"
[[ "$source_count" == "$cli_count" ]] || fail "types count mismatch: source=${source_count} cli=${cli_count}"

pass "types output matches registry source entries"

profiles_output="$("$BIN" profile 2>&1)"
assert_contains "$profiles_output" "batch" "profile output missing batch profile"
assert_contains "$profiles_output" "singleRun" "profile output missing singleRun profile"
pass "profile output lists both supported job profiles"

section "Done"
info "Types integrity test passed"
