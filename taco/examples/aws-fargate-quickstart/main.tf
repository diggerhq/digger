#############################################
# main.tf — ECS (Fargate) + NLB + CloudFront
# Uses variables from variables.tf / *.tfvars
#############################################

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 5.0" }
  }
}

provider "aws" {
  region = var.region
}

locals {
  name = var.name_prefix
}

############################
# Security (tasks SG)
############################
resource "aws_security_group" "tasks" {
  name   = "${local.name}-tasks-sg"
  vpc_id = var.vpc_id

  # Demo: open app port. Lock down in prod.
  ingress {
    protocol    = "tcp"
    from_port   = var.container_port
    to_port     = var.container_port
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    protocol    = "-1"
    from_port   = 0
    to_port     = 0
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${local.name}-tasks-sg" }
}

############################
# Network Load Balancer
############################
resource "aws_lb" "nlb" {
  name               = "${local.name}-nlb"
  load_balancer_type = "network"
  internal           = false
  subnets            = var.public_subnet_ids

  tags = { Name = "${local.name}-nlb" }
}

resource "aws_lb_target_group" "tg" {
  name        = "${local.name}-tg"
  port        = var.container_port
  protocol    = "TCP"
  vpc_id      = var.vpc_id
  target_type = "ip"

  health_check {
    protocol = "TCP"
    port     = "traffic-port"
  }

  tags = { Name = "${local.name}-tg" }
}

resource "aws_lb_listener" "tcp80" {
  load_balancer_arn = aws_lb.nlb.arn
  port              = 80
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.tg.arn
  }
}

############################
# IAM (exec + task roles)
############################
data "aws_iam_policy_document" "task_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

# Execution role: ECR pulls, CloudWatch Logs, etc.
resource "aws_iam_role" "exec" {
  name               = "${local.name}-exec"
  assume_role_policy = data.aws_iam_policy_document.task_assume.json
}

resource "aws_iam_role_policy_attachment" "exec_logs" {
  role       = aws_iam_role.exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# Add Secrets Manager access for the execution role
resource "aws_iam_role_policy_attachment" "exec_secrets" {
  role       = aws_iam_role.exec.name
  policy_arn = "arn:aws:iam::aws:policy/SecretsManagerReadWrite"
}

# Task role: grant app S3 access (bucket + prefix)
resource "aws_iam_role" "task" {
  name               = "${local.name}-task"
  assume_role_policy = data.aws_iam_policy_document.task_assume.json
}

data "aws_iam_policy_document" "s3_policy" {
  statement {
    actions   = ["s3:ListBucket"]
    resources = ["arn:aws:s3:::${var.opentaco_s3_bucket}"]
    condition {
      test     = "StringLike"
      variable = "s3:prefix"
      values   = ["${var.opentaco_s3_prefix}/*"]
    }
  }
  statement {
    actions   = ["s3:GetObject","s3:PutObject","s3:DeleteObject"]
    resources = ["arn:aws:s3:::${var.opentaco_s3_bucket}/${var.opentaco_s3_prefix}/*"]
  }
}

resource "aws_iam_policy" "s3_policy" {
  name   = "${local.name}-s3"
  policy = data.aws_iam_policy_document.s3_policy.json
}

resource "aws_iam_role_policy_attachment" "task_s3" {
  role       = aws_iam_role.task.name
  policy_arn = aws_iam_policy.s3_policy.arn
}

############################
# Logs + Secrets
############################
resource "aws_cloudwatch_log_group" "lg" {
  name              = "/ecs/${local.name}"
  retention_in_days = 7
}

resource "aws_secretsmanager_secret" "auth0_client_secret" {
  name = "${local.name}/auth0_client_secret_v2"
}

resource "aws_secretsmanager_secret_version" "auth0_client_secret_v" {
  secret_id     = aws_secretsmanager_secret.auth0_client_secret.id
  secret_string = var.opentaco_auth_client_secret
}

############################
# ECS Cluster / Task / Service
############################
resource "aws_ecs_cluster" "cluster" {
  name = "${local.name}-cluster"
}

resource "aws_ecs_task_definition" "taskdef" {
  family                   = "${local.name}-task"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = "512"   # 0.5 vCPU
  memory                   = "1024"  # 1 GB
  execution_role_arn       = aws_iam_role.exec.arn
  task_role_arn            = aws_iam_role.task.arn

  container_definitions = jsonencode([
    {
      name        = "web"
      image       = var.container_image
      essential   = true
      portMappings = [{ containerPort = var.container_port, protocol = "tcp" }]

      environment = [
        { name = "OPENTACO_S3_BUCKET",  value = var.opentaco_s3_bucket },
        { name = "OPENTACO_S3_REGION",  value = var.opentaco_s3_region },
        { name = "OPENTACO_S3_PREFIX",  value = var.opentaco_s3_prefix },

        { name = "OPENTACO_AUTH_ISSUER",    value = var.opentaco_auth_issuer },
        { name = "OPENTACO_AUTH_CLIENT_ID", value = var.opentaco_auth_client_id },
        { name = "OPENTACO_AUTH_AUTH_URL",  value = var.opentaco_auth_auth_url },
        { name = "OPENTACO_AUTH_TOKEN_URL", value = var.opentaco_auth_token_url },

        { name = "OPENTACO_PORT",         value = tostring(var.opentaco_port) },
        { name = "OPENTACO_STORAGE",      value = var.opentaco_storage },
        { name = "OPENTACO_AUTH_DISABLE", value = var.opentaco_auth_disable },
        { name = "OPENTACO_PUBLIC_BASE_URL", value = var.opentaco_public_base_url }
      ]

      secrets = [
        { name = "OPENTACO_AUTH_CLIENT_SECRET", valueFrom = aws_secretsmanager_secret.auth0_client_secret.arn }
      ]

      logConfiguration = {
        logDriver = "awslogs",
        options = {
          awslogs-group         = aws_cloudwatch_log_group.lg.name,
          awslogs-region        = var.region,
          awslogs-stream-prefix = "ecs"
        }
      }
    }
  ])
}

resource "aws_ecs_service" "svc" {
  name            = "${local.name}-svc"
  cluster         = aws_ecs_cluster.cluster.id
  task_definition = aws_ecs_task_definition.taskdef.arn
  desired_count   = 1
  launch_type     = "FARGATE"
  platform_version = "LATEST"

  load_balancer {
    target_group_arn = aws_lb_target_group.tg.arn
    container_name   = "web"
    container_port   = var.container_port
  }

  network_configuration {
    subnets          = var.public_subnet_ids   # public subnets → no NAT required
    security_groups  = [aws_security_group.tasks.id]
    assign_public_ip = true
  }

  depends_on = [aws_lb_listener.tcp80]
}

############################
# CloudFront (free *.cloudfront.net HTTPS)
############################
resource "aws_cloudfront_distribution" "edge" {
  enabled     = true
  comment     = "${local.name} via CloudFront"
  price_class = "PriceClass_100"  # US/EU

  origin {
    domain_name = aws_lb.nlb.dns_name
    origin_id   = "nlb-origin"

    # NLB is a public HTTP origin for this demo
    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "http-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    target_origin_id       = "nlb-origin"
    viewer_protocol_policy = "redirect-to-https"

    allowed_methods = ["GET","HEAD","OPTIONS","PUT","POST","PATCH","DELETE"]
    cached_methods  = ["GET","HEAD"]
    compress        = true

    # forward everything; disable caching for dynamic/API
    forwarded_values {
      query_string = true
      headers      = ["*"]
      cookies { forward = "all" }
    }
    min_ttl     = 0
    default_ttl = 0
    max_ttl     = 0
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }

  depends_on = [aws_lb_listener.tcp80]
}

############################
# Outputs
############################
output "cloudfront_domain" {
  description = "Public HTTPS domain for your app"
  value       = aws_cloudfront_distribution.edge.domain_name
}

output "nlb_dns_name" {
  description = "NLB DNS (HTTP origin behind CloudFront)"
  value       = aws_lb.nlb.dns_name
}

output "ecs_cluster" { value = aws_ecs_cluster.cluster.name }
output "ecs_service" { value = aws_ecs_service.svc.name }
