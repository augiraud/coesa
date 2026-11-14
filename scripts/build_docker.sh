#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=./image_ref.sh
. "$(dirname "$0")/image_ref.sh"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Error: the working tree is dirty. Commit first, so the image tag names a commit." >&2
  exit 1
fi

docker build --platform linux/amd64 -t "$IMAGE_LOCAL" -t coesa:latest .
