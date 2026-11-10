variable "workspace_id" {
  type        = string
  description = "Existing workspace UUID. Membership discovery is read-only."
}

variable "model_slug" {
  type        = string
  description = "Existing author/model slug to resolve; no inference is performed."
}

variable "usage_date" {
  type        = string
  description = "Explicit completed UTC day, YYYY-MM-DD, within API retention."
}

variable "lookup_scim_groups" {
  type    = bool
  default = false
}

variable "scim_group_ids" {
  type        = set(string)
  default     = []
  description = "Existing IdP group UUIDs to map. Nonempty values change access."
}

variable "create_destination" {
  type    = bool
  default = false
}

variable "destination_config_json" {
  type        = string
  default     = null
  sensitive   = true
  ephemeral   = true
  description = "Langfuse configuration JSON object. Supply through an ephemeral input or secret source, never a committed tfvars file."
}

variable "destination_config_revision" {
  type    = number
  default = 1
}
