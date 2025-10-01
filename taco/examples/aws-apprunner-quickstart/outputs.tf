output "service_url" {
  description = "App Runner HTTPS service URL"
  value       = aws_apprunner_service.this.service_url
}

