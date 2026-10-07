output "url" {
  value = module.ledger.url
}

output "fqdn" {
  value = module.ledger.fqdn
}

output "auth_token" {
  value     = module.ledger.auth_token
  sensitive = true
}
