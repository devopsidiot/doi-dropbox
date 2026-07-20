output "uploads_bucket_name" {
  description = "Name of the bucket holding uploaded files"
  value       = aws_s3_bucket.uploads.id
}

output "frontend_bucket_name" {
  description = "Name of the website bucket (upload your site here)"
  value       = aws_s3_bucket.frontend.id
}

output "cognito_user_pool_id" {
  description = "User pool ID (needed to create your user)"
  value       = aws_cognito_user_pool.main.id
}

output "cognito_client_id" {
  description = "App client ID (goes in frontend config and CLI settings)"
  value       = aws_cognito_user_pool_client.app.id
}

output "cognito_region" {
  description = "Region the Cognito user pool lives in"
  value       = var.aws_region
}

output "api_base_url" {
  description = "Base URL of the API (goes in frontend config and CLI settings)"
  value       = aws_apigatewayv2_api.main.api_endpoint
}

output "cloudfront_domain" {
  description = "CloudFront's own domain (useful for testing before DNS is ready)"
  value       = aws_cloudfront_distribution.site.domain_name
}
