data "aws_ecr_repository" "repo" {
  name = var.ecr_repo_name
}

locals {
  image_identifier = "${data.aws_ecr_repository.repo.repository_url}:${var.image_tag}"
}

data "aws_iam_policy_document" "apprunner_ecr_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["build.apprunner.amazonaws.com", "tasks.apprunner.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "apprunner_ecr_access" {
  name               = "${var.name_prefix}-apprunner-ecr"
  assume_role_policy = data.aws_iam_policy_document.apprunner_ecr_assume.json
}

resource "aws_iam_role_policy_attachment" "apprunner_ecr_access" {
  role       = aws_iam_role.apprunner_ecr_access.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSAppRunnerServicePolicyForECRAccess"
}

data "aws_iam_policy_document" "task_s3" {
  statement {
    sid    = "ListBucket"
    effect = "Allow"
    actions = ["s3:ListBucket"]
    resources = ["arn:aws:s3:::${var.bucket_name}"]
    condition {
      test     = "StringLike"
      variable = "s3:prefix"
      values   = ["${var.bucket_prefix}/*", "${var.bucket_prefix}"]
    }
  }

  statement {
    sid    = "ObjectRW"
    effect = "Allow"
    actions = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
    resources = ["arn:aws:s3:::${var.bucket_name}/${var.bucket_prefix}/*"]
  }
}

resource "aws_iam_role" "apprunner_instance" {
  name               = "${var.name_prefix}-apprunner-instance"
  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [{
      Effect = "Allow",
      Principal = { Service = "tasks.apprunner.amazonaws.com" },
      Action = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_policy" "task_s3" {
  name   = "${var.name_prefix}-s3"
  policy = data.aws_iam_policy_document.task_s3.json
}

resource "aws_iam_role_policy_attachment" "task_s3" {
  role       = aws_iam_role.apprunner_instance.name
  policy_arn = aws_iam_policy.task_s3.arn
}

resource "aws_apprunner_service" "this" {
  service_name = var.name_prefix

  source_configuration {
    auto_deployments_enabled = true

    image_repository {
      image_repository_type = "ECR"
      image_identifier      = local.image_identifier

      image_configuration {
        port = tostring(var.container_port)
        runtime_environment_variables = {
          OPENTACO_PORT               = tostring(var.container_port)
          OPENTACO_STORAGE            = "s3"
          OPENTACO_S3_BUCKET          = var.bucket_name
          OPENTACO_S3_REGION          = var.aws_region
          OPENTACO_S3_PREFIX          = var.bucket_prefix
          OPENTACO_AUTH_DISABLE       = var.opentaco_auth_disable ? "true" : "false"
          OPENTACO_AUTH_ISSUER        = var.opentaco_auth_issuer
          OPENTACO_AUTH_CLIENT_ID     = var.opentaco_auth_client_id
          OPENTACO_AUTH_CLIENT_SECRET = var.opentaco_auth_client_secret
          OPENTACO_AUTH_AUTH_URL      = var.opentaco_auth_auth_url
          OPENTACO_AUTH_TOKEN_URL     = var.opentaco_auth_token_url
          OPENTACO_PUBLIC_BASE_URL    = var.opentaco_public_base_url
        }
      }
    }

    authentication_configuration {
      access_role_arn = aws_iam_role.apprunner_ecr_access.arn
    }
  }

  instance_configuration {
    instance_role_arn = aws_iam_role.apprunner_instance.arn
  }

  health_check_configuration {
    protocol = "HTTP"
    path     = "/readyz"
  }
}

