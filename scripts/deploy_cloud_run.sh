#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

. ./.env

IMAGE="$GOOGLE_CLOUD_REGION-docker.pkg.dev/$GOOGLE_CLOUD_PROJECT/$ARTIFACT_REGISTRY_REPOSITORY/coesa:latest"

gcloud run deploy coesa \
  --image "$IMAGE" \
  --project "$GOOGLE_CLOUD_PROJECT" \
  --region "$GOOGLE_CLOUD_REGION" \
  --service-account "$CLOUD_RUN_SERVICE_ACCOUNT@$GOOGLE_CLOUD_PROJECT.iam.gserviceaccount.com" \
  --execution-environment gen1 \
  --cpu 0.08 \
  --memory 256Mi \
  --concurrency 1 \
  --min-instances 0 \
  --max-instances 1 \
  --timeout 30 \
  --allow-unauthenticated
