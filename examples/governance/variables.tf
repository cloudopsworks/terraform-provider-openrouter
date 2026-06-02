variable "openrouter_management_key" {
  type        = string
  sensitive   = true
  description = "OpenRouter management API key. Prefer setting OPENROUTER_API_KEY in the environment."
  default     = null
}

variable "teams" {
  description = "Map of teams to govern. The key becomes the workspace/guardrail slug."
  type = map(object({
    name               = string
    default_text_model = string
    monthly_limit_usd  = number
    allowed_providers  = list(string)
  }))

  default = {
    search = {
      name               = "Search Team"
      default_text_model = "openai/gpt-4o"
      monthly_limit_usd  = 200
      allowed_providers  = ["openai", "anthropic"]
    }
    growth = {
      name               = "Growth Team"
      default_text_model = "anthropic/claude-sonnet-4-6"
      monthly_limit_usd  = 100
      allowed_providers  = ["anthropic"]
    }
  }
}
