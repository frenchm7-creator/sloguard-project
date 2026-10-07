terraform {
  required_version = ">= 1.6"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}

# azurerm 4.x requires a subscription. We deliberately avoid hard coding it:
#   export ARM_SUBSCRIPTION_ID=$(az account show --query id -o tsv)
provider "azurerm" {
  features {}
}
