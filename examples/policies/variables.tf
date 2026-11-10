variable "member_ids" {
  description = "Existing organization member user IDs already eligible for this workspace. No assignments are created when empty."
  type        = set(string)
  default     = []
}
