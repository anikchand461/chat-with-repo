#!/usr/bin/env bash
# Build and deploy the ChatWithRepo backend to AWS (template.yaml).
#
#   ./deploy-aws.sh                          # build, push, deploy (asks to confirm the changeset)
#   ./deploy-aws.sh --no-execute-changeset   # build, push, only create the changeset
#
# Extra arguments are passed to `sam deploy`.
#
# 1. Builds the container image (arm64) and pushes it to ECR with a unique
#    tag. The same image is used by the Lambda function and the Fargate
#    indexing task, so they always run the same code.
# 2. Builds the zip function (sam build) and deploys the stack with the new
#    image. Parameters not given here keep their previous values (secrets
#    included), so they are only asked for on the very first deploy.
#
# Needs: docker, aws, sam, uv. Render is not affected by any of this.

set -euo pipefail
cd "$(dirname "$0")"

REGION=ap-south-1
REPO_NAME=chatwithrepo-api

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REGISTRY="$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com"
TAG="$(git rev-parse --short HEAD)-$(date +%Y%m%d%H%M%S)"
IMAGE="$REGISTRY/$REPO_NAME:$TAG"

# ECR repository (created once, outside the stack: the image has to exist
# before the stack that uses it is deployed). Keeps the last 3 images.
if ! aws ecr describe-repositories --region "$REGION" --repository-names "$REPO_NAME" >/dev/null 2>&1; then
  aws ecr create-repository --region "$REGION" --repository-name "$REPO_NAME" \
    --image-scanning-configuration scanOnPush=true >/dev/null
  aws ecr put-lifecycle-policy --region "$REGION" --repository-name "$REPO_NAME" \
    --lifecycle-policy-text '{"rules":[{"rulePriority":1,"description":"keep last 3 images","selection":{"tagStatus":"any","countType":"imageCountMoreThan","countNumber":3},"action":{"type":"expire"}}]}' >/dev/null
  # Lets Lambda pull images from this repository.
  aws ecr set-repository-policy --region "$REGION" --repository-name "$REPO_NAME" \
    --policy-text "{\"Version\":\"2012-10-17\",\"Statement\":[{\"Sid\":\"LambdaECRImageRetrievalPolicy\",\"Effect\":\"Allow\",\"Principal\":{\"Service\":\"lambda.amazonaws.com\"},\"Action\":[\"ecr:BatchGetImage\",\"ecr:GetDownloadUrlForLayer\"],\"Condition\":{\"StringLike\":{\"aws:sourceArn\":\"arn:aws:lambda:$REGION:$ACCOUNT_ID:function:chatwithrepo-*\"}}}]}" >/dev/null
fi

aws ecr get-login-password --region "$REGION" | docker login --username AWS --password-stdin "$REGISTRY"

# --provenance=false: Lambda rejects multi-manifest (attestation) images.
docker build --platform linux/arm64 --provenance=false -t "$IMAGE" .
# Large layers sometimes time out on upload; already-pushed layers are skipped.
for attempt in 1 2 3; do
  docker push "$IMAGE" && break
  [ "$attempt" = 3 ] && exit 1
  echo "Push failed, retrying ($attempt/3)..."
done

sam build
# --image-repositories is required by SAM for image functions; the image is
# already pushed, so it only points at the same repository.
sam deploy --parameter-overrides "ApiImageUri=$IMAGE" \
  --image-repositories "ApiContainerFunction=$REGISTRY/$REPO_NAME" "$@"

echo "Image: $IMAGE"
aws cloudformation describe-stacks --region "$REGION" --stack-name chatwithrepo \
  --query 'Stacks[0].Outputs[].[OutputKey,OutputValue]' --output table

# Warm-up: the first start after a deploy pulls the new image (~20s), which
# is longer than the frontend's primary timeout. Take that hit here instead
# of the first user. Skipped when only a changeset was created.
case " $* " in *" --no-execute-changeset "*) exit 0 ;; esac
URL=$(aws cloudformation describe-stacks --region "$REGION" --stack-name chatwithrepo \
  --query "Stacks[0].Outputs[?OutputKey=='ContainerApiUrl'].OutputValue" --output text)
echo "Warming up $URL ..."
curl -s -o /dev/null --max-time 60 --retry 2 -w "Warm-up: HTTP %{http_code} in %{time_total}s\n" "$URL" \
  || echo "Warm-up request failed (the deploy itself succeeded)."
