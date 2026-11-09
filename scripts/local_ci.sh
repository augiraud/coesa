#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.." || exit 1

fix=false
case "${1:-}" in
  "") ;;
  --fix) fix=true ;;
  -h | --help) echo "Usage: $0 [--fix]"; exit 0 ;;
  *) echo "Unknown option: $1 (see --help)" >&2; exit 2 ;;
esac

# Development tools are pinned in tools/go.mod.
tool() { go tool -modfile=tools/go.mod "$@"; }

failed=""
step() {
  local name=$1
  shift
  echo "==> $name"
  "$@" || failed="$failed"$'\n'"  - $name"
}

if $fix; then
  go mod tidy
  go -C tools mod tidy
  tool golangci-lint fmt
  tool golangci-lint run --fix || true
fi

step "Modules (go mod tidy)" go mod tidy -diff
step "Tool modules (go mod tidy)" go -C tools mod tidy -diff
step "Format (golangci-lint fmt)" tool golangci-lint fmt --diff
step "Lint (golangci-lint)" tool golangci-lint run
step "Tests (go test)" tool gotestsum --jsonfile test-report.json -- -race -coverprofile=coverage.out ./...
step "Vulnerabilities (govulncheck)" tool govulncheck ./...
step "Release config (goreleaser)" tool goreleaser check --quiet

if [ -n "$failed" ]; then
  echo "Failed:$failed"
  exit 1
fi
echo "All checks passed."
