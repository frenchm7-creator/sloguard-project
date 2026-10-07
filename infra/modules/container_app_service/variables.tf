variable "name" {
  description = "Container App name (also prefixes the other resources). Lowercase letters, digits and '-', max 32 chars."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{0,30}[a-z0-9]$", var.name))
    error_message = "name must be 2-32 chars, lowercase alphanumerics or '-', start with a letter and end alphanumeric."
  }
}

variable "resource_group_name" {
  type = string
}

variable "location" {
  type = string
}

variable "image" {
  description = "Fully qualified image WITH an immutable tag, e.g. ghcr.io/me/sloguard:sha-1a2b3c4."
  type        = string

  # A mutable tag makes `terraform apply` a no-op after a new push, so the
  # running revision silently drifts from what Git says is deployed.
  validation {
    condition     = strcontains(var.image, ":") && !endswith(var.image, ":latest")
    error_message = "image must carry an explicit, immutable tag (not ':latest')."
  }
}

variable "port" {
  type    = number
  default = 8080
}

variable "cpu" {
  type    = number
  default = 0.25
}

variable "memory" {
  description = "Must be a valid pairing for cpu on Container Apps (0.25 cpu <-> 0.5Gi)."
  type        = string
  default     = "0.5Gi"
}

variable "min_replicas" {
  description = "Keep >= 1: scale-to-zero would make Prometheus scrapes and the SLI itself disappear."
  type        = number
  default     = 1

  validation {
    condition     = var.min_replicas >= 1
    error_message = "min_replicas must be >= 1 for a service that is scraped and measured against an SLO."
  }
}

variable "max_replicas" {
  type    = number
  default = 3
}

variable "scale_concurrent_requests" {
  description = "Concurrent HTTP requests per replica before scaling out."
  type        = number
  default     = 50
}

variable "enable_fault_injection" {
  description = "Expose /admin/fault (still behind the bearer token). Keep false outside of game days."
  type        = bool
  default     = false
}

variable "log_retention_days" {
  type    = number
  default = 30
}

variable "log_daily_quota_gb" {
  description = "Hard daily ingestion cap on the Log Analytics workspace: a cost guard-rail."
  type        = number
  default     = 0.5
}

variable "tags" {
  type    = map(string)
  default = {}
}
