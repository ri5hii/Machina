#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

BIN="${BIN:-./Machina}"
LOG_DIR="${LOG_DIR:-/tmp/machina-tests}"
mkdir -p "$LOG_DIR"
CONFIG_FILE="config.json"

section() {
  echo
  echo "== $1 =="
}

info() {
  echo "[info] $1"
}

pass() {
  echo "[pass] $1"
}

show_command() {
  echo "[cmd] $*"
}

fail() {
  echo "[fail] $1" >&2
  exit 1
}

require_binary() {
  if [[ ! -x "$BIN" ]]; then
    fail "binary not found at $BIN; build with: go build -o Machina ./cmd"
  fi
}

cleanup_outputs() {
  mkdir -p tests/data/csv/output tests/data/encrypt/output
  rm -f tests/data/csv/output/*.csv
  rm -f tests/data/encrypt/output/*.enc
}

wait_for_health() {
  local port="$1"
  local retries="${2:-30}"
  local i
  for ((i=1; i<=retries; i++)); do
    if curl -sSf "http://localhost:${port}/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

start_server() {
  local port="$1"
  local log_file="$2"
  shift 2
  local extra_flags=("$@")

  rm -f "$log_file"
  show_command "$BIN" start --port "$port" "${extra_flags[@]}"
  "$BIN" start --port "$port" "${extra_flags[@]}" >"$log_file" 2>&1 &
  SERVER_PID=$!
  info "Server PID: $SERVER_PID"

  if ! wait_for_health "$port" 30; then
    tail -n 200 "$log_file" || true
    fail "server did not become healthy on port ${port}"
  fi

  info "Server is healthy on :${port}"
}

stop_server() {
  if [[ -n "${SERVER_PID:-}" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    info "Stopping server PID $SERVER_PID"
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
}

extract_id() {
  tr -d '\n' | sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'
}

poll_job() {
  local port="$1"
  local job_id="$2"
  local timeout_s="${3:-120}"
  local elapsed=0

  while (( elapsed < timeout_s )); do
    local status_json status
    status_json="$(curl -sS "http://localhost:${port}/jobs/${job_id}")"
    status="$(printf '%s' "$status_json" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')"

    if [[ "$status" == "completed" || "$status" == "failed" ]]; then
      printf '%s\n' "$status_json"
      return 0
    fi

    sleep 1
    ((elapsed+=1))
  done

  fail "timed out waiting for job ${job_id} after ${timeout_s}s"
}

http_status() {
  local method="$1"
  local url="$2"
  local body_file="$3"
  shift 3
  curl -sS -o "$body_file" -w "%{http_code}" -X "$method" "$url" "$@"
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    fail "$message"
  fi
}

assert_file_exists() {
  local path="$1"
  [[ -f "$path" ]] || fail "expected file to exist: $path"
}

assert_file_missing() {
  local path="$1"
  [[ ! -e "$path" ]] || fail "expected file to be absent: $path"
}

backup_config() {
  CONFIG_BACKUP="$(mktemp "${LOG_DIR}/config.XXXXXX.json")"
  cp "$CONFIG_FILE" "$CONFIG_BACKUP"
}

restore_config() {
  if [[ -n "${CONFIG_BACKUP:-}" && -f "${CONFIG_BACKUP:-}" ]]; then
    cp "$CONFIG_BACKUP" "$CONFIG_FILE"
    rm -f "$CONFIG_BACKUP"
  fi
}

registered_job_types_from_source() {
  sed -n 's/^[[:space:]]*reg.Register("\([^"]*\)".*/\1/p' internal/registry/payloadConstructor.go | sort
}
