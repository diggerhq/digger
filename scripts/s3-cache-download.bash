#!/bin/bash
set -e

# S3 Cache Download Script
# Downloads terraform/terragrunt provider cache from S3

BUCKET="$1"
PREFIX="$2"
REGION="$3"
CACHE_DIR="$4"

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
mkdir -p "$CACHE_DIR"
echo "Ensuring cache directory exists: $CACHE_DIR"

# Download cache from S3
echo "Restoring cache from S3 bucket: $BUCKET (prefix: $PREFIX, region: $REGION)"
if aws s3 sync "s3://$BUCKET/$PREFIX" "$CACHE_DIR" --region "$REGION" --only-show-errors; then
  CACHED_FILES=$(find "$CACHE_DIR" -type f 2>/dev/null | wc -l)
  echo "Cache restored successfully ($CACHED_FILES files)"

  if [ "$CACHED_FILES" -gt 0 ]; then
    echo "Sample cached artifacts:"
    find "$CACHE_DIR" -type f 2>/dev/null | head -3
  fi

  # This step is necessary for filesystems where noexec is enabled
  echo "Setting permissions for cached artifacts (provider files)"
  if find "$CACHE_DIR" -type f -name 'terraform-provider-*' \
      -exec chmod +x {} \; ; then
    echo "✅ All provider binaries marked executable."
  else
    echo "❌ Failed to set exec bit on one or more providers." >&2
  fi

else
  echo "No existing cache found or failed to restore (this is normal for first run)"
fi

echo "Cache download completed"
