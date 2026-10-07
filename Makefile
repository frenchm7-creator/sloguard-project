SHELL := /bin/bash
PROM_IMAGE := prom/prometheus:v2.55.1
GO_IMAGE   := golang:1.23-alpine
PY_IMAGE   := python:3.12-slim

.DEFAULT_GOAL := help
.PHONY: help init rules rules-check test test-go test-py test-rules up down logs alerts \
        chaos-errors chaos-latency chaos-reset loadgen-azure tf-fmt tf-validate

help: ## list targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*## "}{printf "  %-16s %s\n",$$1,$$2}'

init: ## create the placeholder token file Prometheus expects (idempotent)
	@test -f observability/prometheus/secrets/azure_metrics_token || printf unset > observability/prometheus/secrets/azure_metrics_token
	@chmod 644 observability/prometheus/secrets/azure_metrics_token

rules: ## regenerate Prometheus rules from slo/slos.yaml
	docker run --rm -v "$(CURDIR)":/w -w /w $(PY_IMAGE) sh -c "pip install -q pyyaml && python tools/gen_rules.py"

rules-check: ## fail if committed rules are stale
	docker run --rm -v "$(CURDIR)":/w -w /w $(PY_IMAGE) sh -c "pip install -q pyyaml && python tools/gen_rules.py --check"

test: test-go test-py test-rules ## run every test suite (needs only Docker)

test-go: ## Go unit tests
	docker run --rm -v "$(CURDIR)/app":/src -w /src $(GO_IMAGE) sh -c "go vet ./... && go test ./..."

test-py: ## generator tests
	docker run --rm -v "$(CURDIR)":/w -w /w $(PY_IMAGE) sh -c "pip install -q pyyaml && python -m unittest discover -s tools -v"

test-rules: ## lint + unit-test the alert rules with promtool
	docker run --rm -v "$(CURDIR)/observability/prometheus":/p --entrypoint promtool $(PROM_IMAGE) check rules /p/rules/slo.rules.yml
	docker run --rm -v "$(CURDIR)/observability/prometheus":/p --entrypoint promtool $(PROM_IMAGE) test rules /p/tests/slo.rules.test.yml

up: init ## build and start the local stack
	docker compose up -d --build
	@echo "Grafana http://localhost:3000 | Prometheus http://localhost:9090/alerts | Alertmanager http://localhost:9093"

down: ## stop and remove the local stack
	docker compose down

logs: ## follow service logs
	docker compose logs -f --tail=50 ledger alertsink

alerts: ## follow ONLY what the "pager" receives
	docker compose logs -f --no-log-prefix alertsink | grep --line-buffered ALERT

chaos-errors: ## inject 5% 5xx errors (burn rate ~50x)
	curl -fsS -X POST 'http://localhost:8080/admin/fault?error_rate=0.05'; echo

chaos-latency: ## add 450ms latency to every request (breaches the 300ms SLO)
	curl -fsS -X POST 'http://localhost:8080/admin/fault?latency_ms=450'; echo

chaos-reset: ## remove all injected faults
	curl -fsS -X POST 'http://localhost:8080/admin/fault?reset=true'; echo

loadgen-azure: ## send traffic to the Azure deployment (after scripts/attach-azure.sh)
	docker compose run --rm --no-deps -e TARGET=$$(terraform -chdir=infra/envs/dev output -raw url) -e RPS=10 loadgen

tf-fmt: ## format Terraform
	terraform fmt -recursive infra

tf-validate: ## validate Terraform without touching Azure
	terraform -chdir=infra/envs/dev init -backend=false -input=false && terraform -chdir=infra/envs/dev validate
