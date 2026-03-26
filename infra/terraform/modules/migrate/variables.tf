variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "name_prefix" {
  type = string
}

variable "api_service_account" {
  description = "Service account email for the API migrate job"
  type        = string
}

variable "auth_service_account" {
  description = "Service account email for the Auth migrate job"
  type        = string
}
