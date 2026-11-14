#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=./image_ref.sh
. "$(dirname "$0")/image_ref.sh"
require_image_registry

gcloud run deploy coesa \
  --image "$IMAGE_REGISTRY" \
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
