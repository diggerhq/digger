#!/bin/bash

# Digger HTTP State Backend Runner
# This script sets up the environment and runs the state backend

set -e

# Check if required environment variables are set
if [ -z "$S3_BUCKET" ]; then
    echo "Error: S3_BUCKET environment variable is required"
    echo "Usage: S3_BUCKET=your-bucket-name ./run.sh"
    exit 1
fi

# Set default values for optional variables
export AWS_REGION=${AWS_REGION:-"us-east-1"}
export PORT=${PORT:-"8080"}

echo "Starting Digger HTTP State Backend..."
echo "S3 Bucket: $S3_BUCKET"
echo "AWS Region: $AWS_REGION"
echo "Port: $PORT"

# Run the application
go run . 