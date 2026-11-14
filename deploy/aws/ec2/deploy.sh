#!/usr/bin/env sh
set -eu

: "${AWS_REGION:?AWS_REGION is required}"
: "${AWS_ACCOUNT_ID:?AWS_ACCOUNT_ID is required}"
: "${IMAGE_URI:?IMAGE_URI is required}"
: "${API_DOMAIN:?API_DOMAIN is required}"

ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
DEPLOY_DIR="/opt/whisperledger"

aws ecr get-login-password --region "$AWS_REGION" \
  | docker login --username AWS --password-stdin "$ECR_REGISTRY"

export IMAGE_URI API_DOMAIN
cd "$DEPLOY_DIR"
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d --remove-orphans

for attempt in 1 2 3 4 5 6; do
  if curl --fail --silent --show-error http://127.0.0.1/healthz >/dev/null; then
    docker image prune -af --filter "until=168h"
    exit 0
  fi
  sleep 5
done

echo "WhisperLedger API did not become healthy" >&2
exit 1
