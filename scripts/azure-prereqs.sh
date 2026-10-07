#!/usr/bin/env bash
# Run once per subscription. Registers the resource providers the container app needs
# and prints the env var Terraform's azurerm 4.x provider requires.
set -euo pipefail
command -v az >/dev/null || { echo "Azure CLI not found: https://learn.microsoft.com/cli/azure/install-azure-cli"; exit 1; }
az account show >/dev/null 2>&1 || az login
for ns in Microsoft.App Microsoft.OperationalInsights; do
  echo "registering $ns (can take a couple of minutes the first time)..."
  az provider register --namespace "$ns" --wait
done
SUB=$(az account show --query id -o tsv)
echo
echo "Done. In EVERY terminal you run terraform from, set:"
echo "  export ARM_SUBSCRIPTION_ID=$SUB"
