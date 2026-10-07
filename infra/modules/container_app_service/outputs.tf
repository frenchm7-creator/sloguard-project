output "fqdn" {
  description = "Public hostname of the app (no scheme)."
  value       = azurerm_container_app.this.ingress[0].fqdn
}

output "url" {
  value = "https://${azurerm_container_app.this.ingress[0].fqdn}"
}

output "auth_token" {
  description = "Bearer token protecting /metrics (and /admin/*). Also stored in Terraform state - see README 'Limitations'."
  value       = random_password.auth_token.result
  sensitive   = true
}

output "log_analytics_workspace_id" {
  value = azurerm_log_analytics_workspace.this.id
}
