variable "aws_region" {
  description = "The main AWS region to deploy into"
  type        = string
  default     = "us-east-1"
}

variable "domain_name" {
  description = "Your domain name"
  type        = string
  default     = "devopsidiot.com"
}

variable "uploads_bucket_name" {
  description = "Globally-unique name for the bucket that holds uploaded files"
  type        = string
}

variable "frontend_bucket_name" {
  description = "Globally-unique name for the bucket that holds the website files"
  type        = string
}

variable "route53_zone_id" {
  description = "The ID of your existing Route53 hosted zone for the domain"
  type        = string
}

variable "notification_email" {
  description = "Email used for Cognito account recovery"
  type        = string
}
