variable "aws_region" {
  description = "The main AWS region to deploy into"
  type        = string
  default     = "us-east-1"
}

variable "domain_name" {
  # The hostname this app is served at — NOT the apex of the zone. This stack
  # creates its own CloudFront distribution and a Route 53 A record at exactly
  # this name, so pointing it at the apex takes over the domain and replaces
  # whatever is already published there.
  description = "Hostname to serve the dropbox at. Must not be the zone apex."
  type        = string
  default     = "dropbox.devopsidiot.com"

  validation {
    # A subdomain has at least three labels. This will not catch every way of
    # naming the apex, but it catches the one that actually happens: leaving
    # the default at, or pasting in, the bare registered domain.
    condition     = length(split(".", var.domain_name)) >= 3
    error_message = "domain_name must be a subdomain (e.g. dropbox.example.com), not the zone apex."
  }
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
