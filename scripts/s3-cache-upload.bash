#!/bin/bash
set -e

# S3 Cache Upload Script
# Uploads terraform/terragrunt provider cache to S3

BUCKET="$1"
PREFIX="$2"
REGION="$3"
CACHE_DIR="$4"

# Validation
if [ -z "$BUCKET" ]; then
  echo "Error: S3 bucket name is required"
  exit 1
fi

if [ -z "$PREFIX" ]; then
  echo "Error: S3 bucket prefix is required"
  exit 1
fi

if [[ "$PREFIX" == /* ]]; then
  echo "Error: S3 bucket prefix should not start with a leading slash"
  exit 1
fi

if [ -z "$REGION" ]; then
  echo "Error: AWS region is required"
  exit 1
fi

if [ -z "$CACHE_DIR" ]; then
  echo "Error: Cache directory path is required"
  exit 1
fi

# Check if cache directory exists and has files
if [ ! -d "$CACHE_DIR" ]; then
  echo "No cache directory found at: $CACHE_DIR"
  echo "Nothing to upload"
  exit 0
fi

ARTIFACT_COUNT=$(find "$CACHE_DIR" -type f 2>/dev/null | wc -l)

if [ "$ARTIFACT_COUNT" -eq 0 ]; then
  echo "No files to cache - cache directory is empty"
  echo "Nothing to upload"
  exit 0
fi

# Check if AWS CLI is available and credentials are configured
if ! command -v aws &> /dev/null; then
  echo "Error: AWS CLI is not installed or not in PATH"
  exit 1
fi

# Verify AWS credentials are available
if ! aws sts get-caller-identity --region "$REGION" &> /dev/null; then
  echo "Error: AWS credentials are not properly configured or are invalid"
  echo "Please ensure AWS credentials are set up correctly before running this script"
  exit 1
fi

# Upload cache to S3
echo "Saving cache to S3 bucket: $BUCKET (prefix: $PREFIX, region: $REGION)"
echo "Uploading $ARTIFACT_COUNT files"

if aws s3 sync "$CACHE_DIR" "s3://$BUCKET/$PREFIX" --region "$REGION"; then
  echo "Cache saved successfully"
else
  echo "Warning: Failed to save cache (this won't fail the build)"
  exit 0  # Don't fail the build on cache upload failure
fi

echo "Cache upload completed"
