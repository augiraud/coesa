#!/usr/bin/env bash
# shellcheck disable=SC2034 # the variables below are read by the scripts that source this one
#
# Sourced by the Docker scripts so the image name is written once. The caller sets
# set -euo pipefail. Building is possible without .env; pushing and deploying are not.

cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit

if ! IMAGE_TAG="$(git rev-parse --short HEAD 2>/dev/null)"; then
  echo "Error: this is not a git checkout, so an image cannot be tagged with a commit." >&2
  exit 1
fi
IMAGE_LOCAL="coesa:${IMAGE_TAG}"

if [[ -f .env ]]; then
  . ./.env
  IMAGE_REGISTRY="$GOOGLE_CLOUD_REGION-docker.pkg.dev/$GOOGLE_CLOUD_PROJECT/$ARTIFACT_REGISTRY_REPOSITORY/coesa:${IMAGE_TAG}"
else
  IMAGE_REGISTRY=""
fi

require_image_registry() {
  if [[ -z "$IMAGE_REGISTRY" ]]; then
    echo "Error: .env not found. Copy .env.template to .env and fill in your registry values." >&2
    exit 1
  fi
}
