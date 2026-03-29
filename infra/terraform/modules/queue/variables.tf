variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "name_prefix" {
  type = string
}

variable "max_concurrent_dispatches" {
  description = "Maximum concurrent task dispatches"
  type        = number
  default     = 3
}

variable "max_dispatches_per_second" {
  description = "Maximum task dispatches per second"
  type        = number
  default     = 1
}
