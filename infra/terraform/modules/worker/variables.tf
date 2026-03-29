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

variable "github_app_id" {
  type = string
}

variable "gcs_bucket" {
  type = string
}

variable "api_url" {
  description = "API base URL for worker callbacks"
  type        = string
}

variable "cpu" {
  description = "Worker CPU allocation"
  type        = string
  default     = "1"
}

variable "memory" {
  description = "Worker memory allocation"
  type        = string
  default     = "2Gi"
}

variable "max_instances" {
  description = "Maximum worker instances"
  type        = number
  default     = 2
}

variable "timeout" {
  description = "Worker request timeout"
  type        = string
  default     = "900s"
}
