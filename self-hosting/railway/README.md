# Railway self-hosting

Use the OpenTaco Railway template to provision the full stack, then configure the required variables and integration flows.

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/FIg15a?referralCode=XA06uX&utm_medium=integration&utm_source=template&utm_campaign=generic)

## Quick flow

1. Start from the template.
2. Set pre-deploy values:
   - WorkOS values for the UI service
   - `GITHUB_ORG` for the orchestrator service
3. Provision services in Railway.
4. Configure a public domain (Railway-generated or custom CNAME).
5. Set remaining required environment variables using the self-hosting configuration guide.
6. Redeploy and verify service access.
7. Complete GitHub App setup.

## Verify after setup

- Storage via the Units page
- Remote runs (if configured and enabled)
- PR automation and drift workflows

## More detail

For the full guide, see https://docs.opentaco.dev/self-hosting/railway.
