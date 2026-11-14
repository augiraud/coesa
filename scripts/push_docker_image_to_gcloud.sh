#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ ! -f .env ]]; then
  echo "Error: .env not found. Copy .env.template to .env and fill in your registry values." >&2
  exit 1
fi

. ./.env

IMAGE="$GOOGLE_CLOUD_REGION-docker.pkg.dev/$GOOGLE_CLOUD_PROJECT/$ARTIFACT_REGISTRY_REPOSITORY/coesa:latest"

docker tag coesa:latest "$IMAGE"
docker push "$IMAGE"
