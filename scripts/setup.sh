#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

for cmd in git go; do
  command -v "$cmd" >/dev/null || { echo "Error: $cmd is required. Install it, then run this script again." >&2; exit 1; }
done

go mod download
go -C tools mod download

if [[ -z "${CI:-}" ]]; then
  git config core.hooksPath .githooks
fi

echo "Setup complete. Check the library with ./scripts/local_ci.sh"
