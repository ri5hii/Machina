#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

echo "Cleaning test outputs..."

mkdir -p tests/data/csv/output tests/data/encrypt/output

CSV_REMOVED=0
ENC_REMOVED=0

if compgen -G "tests/data/csv/output/*.csv" >/dev/null; then
  CSV_REMOVED=$(find tests/data/csv/output -type f -name "*.csv" | wc -l | tr -d ' ')
  rm -f tests/data/csv/output/*.csv
fi

if compgen -G "tests/data/encrypt/output/*.enc" >/dev/null; then
  ENC_REMOVED=$(find tests/data/encrypt/output -type f -name "*.enc" | wc -l | tr -d ' ')
  rm -f tests/data/encrypt/output/*.enc
fi

echo "Removed CSV outputs: ${CSV_REMOVED}"
echo "Removed encrypted outputs: ${ENC_REMOVED}"
echo "Cleanup complete."
