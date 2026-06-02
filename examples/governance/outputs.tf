output "workspace_ids" {
  description = "Workspace UUID per team."
  value       = { for k, ws in openrouter_workspace.team : k => ws.id }
}

output "api_keys" {
  description = "Issued API key secrets per team (only returned on create)."
  value       = { for k, key in openrouter_api_key.team : k => key.key }
  sensitive   = true
}
