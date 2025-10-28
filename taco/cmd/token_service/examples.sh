#!/bin/bash

# Token Service API Examples
# Make sure the token service is running on localhost:8081

set -e

BASE_URL="${TOKEN_SERVICE_URL:-http://localhost:8081}"
USER_ID="user-123"
ORG_ID="org-456"

echo "============================================"
echo "Token Service API Examples"
echo "============================================"
echo ""

# Health Check
echo "1. Health Check"
echo "   GET $BASE_URL/healthz"
curl -s "$BASE_URL/healthz" | jq '.'
echo ""
echo ""

# Create a Token
echo "2. Create Token"
echo "   POST $BASE_URL/api/v1/tokens"
TOKEN_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/tokens" \
  -H "Content-Type: application/json" \
  -d "{
    \"user_id\": \"$USER_ID\",
    \"org_id\": \"$ORG_ID\",
    \"name\": \"Example API Token\",
    \"expires_in\": \"720h\"
  }")

echo "$TOKEN_RESPONSE" | jq '.'
TOKEN_ID=$(echo "$TOKEN_RESPONSE" | jq -r '.id')
TOKEN_VALUE=$(echo "$TOKEN_RESPONSE" | jq -r '.token')
echo ""
echo "   Created token ID: $TOKEN_ID"
echo "   Token value: $TOKEN_VALUE"
echo ""
echo ""

# List All Tokens
echo "3. List All Tokens"
echo "   GET $BASE_URL/api/v1/tokens"
curl -s "$BASE_URL/api/v1/tokens" | jq '.'
echo ""
echo ""

# List Tokens by User
echo "4. List Tokens by User ID"
echo "   GET $BASE_URL/api/v1/tokens?user_id=$USER_ID"
curl -s "$BASE_URL/api/v1/tokens?user_id=$USER_ID" | jq '.'
echo ""
echo ""

# List Tokens by Org
echo "5. List Tokens by Org ID"
echo "   GET $BASE_URL/api/v1/tokens?org_id=$ORG_ID"
curl -s "$BASE_URL/api/v1/tokens?org_id=$ORG_ID" | jq '.'
echo ""
echo ""

# Get Specific Token
echo "6. Get Token by ID"
echo "   GET $BASE_URL/api/v1/tokens/$TOKEN_ID"
curl -s "$BASE_URL/api/v1/tokens/$TOKEN_ID" | jq '.'
echo ""
echo ""

# Verify Token
echo "7. Verify Token"
echo "   POST $BASE_URL/api/v1/tokens/verify"
curl -s -X POST "$BASE_URL/api/v1/tokens/verify" \
  -H "Content-Type: application/json" \
  -d "{
    \"token\": \"$TOKEN_VALUE\",
    \"user_id\": \"$USER_ID\",
    \"org_id\": \"$ORG_ID\"
  }" | jq '.'
echo ""
echo ""

# Delete Token
echo "8. Delete Token"
echo "   DELETE $BASE_URL/api/v1/tokens/$TOKEN_ID"
curl -s -X DELETE "$BASE_URL/api/v1/tokens/$TOKEN_ID" | jq '.'
echo ""
echo ""

# Verify Deleted Token (should fail)
echo "9. Verify Deleted Token (should fail)"
echo "   POST $BASE_URL/api/v1/tokens/verify"
curl -s -X POST "$BASE_URL/api/v1/tokens/verify" \
  -H "Content-Type: application/json" \
  -d "{
    \"token\": \"$TOKEN_VALUE\"
  }" | jq '.'
echo ""
echo ""

echo "============================================"
echo "Examples Complete!"
echo "============================================"

