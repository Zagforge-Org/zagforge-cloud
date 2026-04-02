variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "name_prefix" {
  type = string
}

variable "environment" {
  type = string
}

variable "min_instances" {
  type    = number
  default = 0
}

variable "max_instances" {
  type    = number
  default = 2
}

variable "jwt_issuer" {
  description = "JWT issuer URL (e.g. https://auth.zagforge.com)"
  type        = string
}

variable "oauth_callback_base_url" {
  description = "Base URL for OAuth callbacks (e.g. https://auth.zagforge.com)"
  type        = string
}

variable "cors_allowed_origins" {
  type    = string
  default = ""
}
