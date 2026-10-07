#!/usr/bin/env bash
# Point the local Prometheus to azure deployment - writes the scrape target
# (file_sd) and the bearer token file so prometheus picks both up without a restart.
set -euo pipefail
cd "$(dirname "$0")/.."
TF=infra/envs/dev
fqdn=$(terraform -chdir="$TF" output -raw fqdn)
token=$(terraform -chdir="$TF" output -raw auth_token)

umask 022   # the prometheus container runs as 'nobody' and must be able to read these (local dev only)
printf '%s' "$token" > observability/prometheus/secrets/azure_metrics_token
printf '[{"targets":["%s:443"],"labels":{"env":"azure"}}]\n' "$fqdn" > observability/prometheus/targets/azure.json

echo "Attached https://$fqdn. Within ~30s http://localhost:9090/targets should show ledger-azure UP."
echo "Send it traffic:  make loadgen-azure"
