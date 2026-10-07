set -euo pipefail
LOC="${LOCATION:-westus3}"
RG="rg-sloguard-tfstate"
SA="sloguardtf$(openssl rand -hex 4)"   # globally unique, <=24 lowercase alphanumerics

az group create -n "$RG" -l "$LOC" -o none
az storage account create -n "$SA" -g "$RG" -l "$LOC" --sku Standard_LRS \
  --min-tls-version TLS1_2 --allow-blob-public-access false --allow-shared-key-access false -o none
az storage account blob-service-properties update --account-name "$SA" -g "$RG" \
  --enable-versioning true --enable-delete-retention true --delete-retention-days 14 -o none
# Container via the ARM control plane: works before the data-plane role below propagates.
az storage container-rm create --storage-account "$SA" -g "$RG" --name tfstate -o none
az role assignment create --role "Storage Blob Data Contributor" \
  --assignee-object-id "$(az ad signed-in-user show --query id -o tsv)" --assignee-principal-type User \
  --scope "$(az storage account show -n "$SA" -g "$RG" --query id -o tsv)" -o none

echo
echo "State backend ready. Put this in infra/envs/dev/backend.tf (copy backend.tf.example):"
echo "  storage_account_name = \"$SA\""
echo "Role assignments can take ~1-2 minutes to propagate before 'terraform init' works."
