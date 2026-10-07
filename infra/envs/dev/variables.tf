variable "image" {
  description = "Image with immutable tag, e.g. ghcr.io/<you>/sloguard:sha-1a2b3c4 (see README step 'Deploy')."
  type        = string
}

variable "location" {
  description = "Azure region that supports Container Apps."
  type        = string
  default     = "westus3"
}

variable "environment" {
  type    = string
  default = "dev"
}

variable "enable_fault_injection" {
  type    = bool
  default = false
}
