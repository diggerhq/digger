#!/usr/bin/env bash
set -euo pipefail

# Mirrors the OpenTaco Statesman image from GHCR to your ECR repo
# Required env: AWS_REGION (default: us-east-1)
# Optional env: ECR_REPO_NAME (default: opentaco-statesman), IMAGE_TAG (default: latest)

AWS_REGION=${AWS_REGION:-us-east-1}
ECR_REPO_NAME=${ECR_REPO_NAME:-opentaco-statesman}
IMAGE_TAG=${IMAGE_TAG:-latest}
SRC_IMAGE="ghcr.io/diggerhq/digger/taco-statesman:${IMAGE_TAG}"

echo "Region: ${AWS_REGION}"
echo "ECR repo: ${ECR_REPO_NAME}"
echo "Image tag: ${IMAGE_TAG}"

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_REGISTRY="${ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
DEST_IMAGE="${ECR_REGISTRY}/${ECR_REPO_NAME}:${IMAGE_TAG}"

echo "Ensuring ECR repository exists..."
aws ecr describe-repositories --repository-names "${ECR_REPO_NAME}" --region "${AWS_REGION}" >/dev/null 2>&1 || \
  aws ecr create-repository --repository-name "${ECR_REPO_NAME}" --region "${AWS_REGION}" >/dev/null

echo "Logging in to ECR..."
aws ecr get-login-password --region "${AWS_REGION}" | docker login --username AWS --password-stdin "${ECR_REGISTRY}"

echo "Pulling source image ${SRC_IMAGE}..."
docker pull --platform linux/amd64 "${SRC_IMAGE}"

echo "Tagging for ECR as ${DEST_IMAGE}..."
docker tag "${SRC_IMAGE}" "${DEST_IMAGE}"

echo "Pushing to ECR..."
docker push "${DEST_IMAGE}"

echo "Done. Use this image in Terraform:"
echo "  ${DEST_IMAGE}"

