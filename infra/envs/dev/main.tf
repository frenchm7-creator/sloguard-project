locals {
  app_name = "sloguard-${var.environment}"
  tags = {
    project     = "sloguard"
    environment = var.environment
    managed_by  = "terraform"
  }
}

resource "azurerm_resource_group" "this" {
  name     = "rg-${local.app_name}"
  location = var.location
  tags     = local.tags
}

module "ledger" {
  source = "../../modules/container_app_service"

  name                   = local.app_name
  resource_group_name    = azurerm_resource_group.this.name
  location               = azurerm_resource_group.this.location
  image                  = var.image
  enable_fault_injection = var.enable_fault_injection
  tags                   = local.tags
}
