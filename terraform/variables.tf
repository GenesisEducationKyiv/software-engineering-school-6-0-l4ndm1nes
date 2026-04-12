variable "aws_region" {
  default = "eu-central-1"
}

variable "project_name" {
  default = "gh-release-notifier"
}

variable "db_password" {
  type      = string
  sensitive = true
}

variable "github_token" {
  type      = string
  sensitive = true
}

variable "api_key" {
  type      = string
  sensitive = true
}

variable "ec2_key_name" {
  description = "Name of the EC2 key pair to use for SSH"
  type        = string
  default     = "gh-release-notifier-key"
}

variable "app_domain" {
  description = "Domain for the application (used in BASE_URL and SES)"
  type        = string
  default     = "releases-api.app"
}
