#!/usr/bin/env bash
# Query the deployed Cloud Run service at one altitude in metres, 11000 by default.
# Usage: scripts/query_cloud_run.sh [altitude_m]
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ ! -f .env ]]; then
  echo "Error: .env not found. Copy .env.template to .env and fill in your registry values." >&2
  exit 1
fi
# shellcheck source=/dev/null
. ./.env

altitude="${1:-11000}"

url="$(gcloud run services describe coesa \
  --project "$GOOGLE_CLOUD_PROJECT" \
  --region "$GOOGLE_CLOUD_REGION" \
  --format='value(status.url)')"
if [[ -z "$url" ]]; then
  echo "Error: the coesa service has no URL. Has it been deployed?" >&2
  exit 1
fi

curl --silent --show-error --fail-with-body \
  --get --data-urlencode "altitude_m=$altitude" "$url/v1/coesa"
