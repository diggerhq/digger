#!/bin/bash
set -e

# S3 Cache Download Script
# Downloads terraform/terragrunt provider cache from S3

BUCKET="$1"
REGION="$2"
CACHE_DIR="$3"

# Validation
if [ -z "$BUCKET" ]; then
  echo "Error: S3 bucket name is required"
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

# Ensure cache directory exists
if [ ! -d "$CACHE_DIR" ]; then
  echo "Creating cache directory: $CACHE_DIR"
  mkdir -p "$CACHE_DIR"
fi

# Download cache from S3
echo "Restoring cache from S3 bucket: $BUCKET (region: $REGION)"
if aws s3 sync "s3://$BUCKET" "$CACHE_DIR" --region "$REGION" --quiet 2>/dev/null; then
  CACHED_FILES=$(find "$CACHE_DIR" -type f 2>/dev/null | wc -l)
  echo "Cache restored successfully ($CACHED_FILES files)"
  
  if [ "$CACHED_FILES" -gt 0 ]; then
    echo "Sample cached artifacts:"
    find "$CACHE_DIR" -type f 2>/dev/null | head -3
  fi
else
  echo "No existing cache found or failed to restore (this is normal for first run)"
fi

echo "Cache download completed"
