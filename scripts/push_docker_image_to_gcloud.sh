#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=./image_ref.sh
. "$(dirname "$0")/image_ref.sh"
require_image_registry

docker tag "$IMAGE_LOCAL" "$IMAGE_REGISTRY"
docker push "$IMAGE_REGISTRY"
