AWS App Runner Quickstart for OpenTaco Statesman (HTTPS out of the box)

This example deploys the OpenTaco Statesman container to AWS App Runner. App Runner gives you a public HTTPS URL automatically — no custom domain or ACM setup required. It uses S3 for state storage via an App Runner instance role.

What it uses/creates:
- ECR repository (you push the image to it with a helper script)
- App Runner service with managed HTTPS domain (e.g., https://xxxx.awsapprunner.com)
- IAM roles: App Runner ECR access role and an instance role with scoped S3 permissions

Prerequisites:
- Terraform >= 1.4, AWS provider ~> 5.0
- AWS CLI and Docker installed and logged in
- Existing S3 bucket and a prefix for OpenTaco state

Step 1 — Mirror the image to ECR (copy/paste)
App Runner pulls images from ECR. Run these commands (region: us-east-1, repo: opentaco-statesman):

```bash
aws ecr create-repository --repository-name opentaco-statesman --region us-east-1

aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin \
  $(aws sts get-caller-identity --query Account --output text).dkr.ecr.us-east-1.amazonaws.com

docker pull --platform linux/amd64 ghcr.io/diggerhq/digger/taco-statesman:latest

docker tag ghcr.io/diggerhq/digger/taco-statesman:latest \
  $(aws sts get-caller-identity --query Account --output text).dkr.ecr.us-east-1.amazonaws.com/opentaco-statesman:latest

docker push \
  $(aws sts get-caller-identity --query Account --output text).dkr.ecr.us-east-1.amazonaws.com/opentaco-statesman:latest
```

Terraform defaults use `ecr_repo_name = "opentaco-statesman"` and `image_tag = "latest"`, so no extra configuration is needed if you keep the commands as is.

Step 2 — Configure Terraform
Create a `terraform.tfvars` file:

```hcl
aws_region    = "us-east-1"
bucket_name   = "your-s3-bucket"
bucket_prefix = "opentaco"
ecr_repo_name = "opentaco-statesman"
image_tag     = "latest"

# Start with auth disabled to get your service URL first
opentaco_auth_disable = true

# Later you can enable OIDC (Auth0 example):
# opentaco_auth_issuer        = "https://your-tenant.auth0.com/"  # trailing slash required
# opentaco_auth_client_id     = "your_client_id"
# opentaco_auth_client_secret = "your_client_secret"
# opentaco_auth_auth_url      = "https://your-tenant.auth0.com/authorize"
# opentaco_auth_token_url     = "https://your-tenant.auth0.com/oauth/token"
```

Step 3 — Deploy

```bash
terraform init
terraform apply -auto-approve
```

Outputs:
- `service_url` – HTTPS base URL (App Runner-managed domain)

Step 4 — Health check

```bash
curl $(terraform output -raw service_url)/readyz
```

Expected:

```json
{"service":"opentaco","status":"ok"}
```

Step 5 — (Optional) Enable SSO
Update `terraform.tfvars` with your OIDC settings and set `opentaco_public_base_url` to the `service_url` you got above, then apply again:

```hcl
opentaco_public_base_url = "https://xxxxxxxx.us-east-1.awsapprunner.com"
opentaco_auth_disable    = false
opentaco_auth_issuer        = "https://your-tenant.auth0.com/"
opentaco_auth_client_id     = "your_client_id"
opentaco_auth_client_secret = "your_client_secret"
opentaco_auth_auth_url      = "https://your-tenant.auth0.com/authorize"
opentaco_auth_token_url     = "https://your-tenant.auth0.com/oauth/token"
```

Then add the callback URL to your IdP:

```
[SERVICE_URL]/oauth/oidc-callback
```

Notes:
- App Runner provides a managed HTTPS endpoint by default; no custom domain or cert is required.
- The instance role is scoped to `s3://<bucket>/<prefix>/*` and ListBucket on your bucket.
- You can later attach a custom domain in App Runner if you want, but it’s optional.
