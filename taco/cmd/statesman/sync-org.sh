#!/bin/bash

SECRET=topsecret                      # must match OPENTACO_ENABLE_INTERNAL_ENDPOINTS
ORG_ID=org_xxx                        # your WorkOS org id
ORG_NAME=my-org                       # internal org name/slug
ORG_DISPLAY="My Org"                  # display name
USER_ID=user_xxx                      # your WorkOS user id
USER_EMAIL=you@example.com            # your email

# create/sync org
curl -s -X POST http://localhost:8080/internal/api/orgs/sync \
  -H "Authorization: Bearer $SECRET" \
  -H "X-User-ID: $USER_ID" -H "X-Email: $USER_EMAIL" \
  -H "Content-Type: application/json" \
  -d '{"name":"'"$ORG_NAME"'","display_name":"'"$ORG_DISPLAY"'","external_org_id":"'"$ORG_ID"'"}'

echo ""
echo "Org synced. Now syncing user..."

# ensure user exists in that org
curl -s -X POST http://localhost:8080/internal/api/users \
  -H "Authorization: Bearer $SECRET" \
  -H "X-Org-ID: $ORG_ID" -H "X-User-ID: $USER_ID" -H "X-Email: $USER_EMAIL" \
  -H "Content-Type: application/json" \
  -d '{"subject":"'"$USER_ID"'","email":"'"$USER_EMAIL"'"}'

echo ""
echo "Done!"
