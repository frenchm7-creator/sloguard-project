# Reusable module: one HTTP service on Azure Container Apps with logging
# health probes, HTTP autoscaling and a generated bearer token for the /metrics.

resource "random_password" "auth_token" {
  length  = 40
  special = false
}

resource "azurerm_log_analytics_workspace" "this" {
  name                = "log-${var.name}"
  location            = var.location
  resource_group_name = var.resource_group_name
  sku                 = "PerGB2018"
  retention_in_days   = var.log_retention_days
  daily_quota_gb      = var.log_daily_quota_gb
  tags                = var.tags
}

resource "azurerm_container_app_environment" "this" {
  name                       = "cae-${var.name}"
  location                   = var.location
  resource_group_name        = var.resource_group_name
  log_analytics_workspace_id = azurerm_log_analytics_workspace.this.id
  tags                       = var.tags
}

resource "azurerm_container_app" "this" {
  name                         = var.name
  container_app_environment_id = azurerm_container_app_environment.this.id
  resource_group_name          = var.resource_group_name
  revision_mode                = "Single"
  tags                         = var.tags

  secret {
    name  = "auth-token"
    value = random_password.auth_token.result
  }

  template {
    min_replicas = var.min_replicas
    max_replicas = var.max_replicas

    http_scale_rule {
      name                = "http-concurrency"
      concurrent_requests = tostring(var.scale_concurrent_requests)
    }

    container {
      name   = "app"
      image  = var.image
      cpu    = var.cpu
      memory = var.memory

      env {
        name  = "LISTEN_ADDR"
        value = ":${var.port}"
      }
      env {
        name  = "ENABLE_FAULT_INJECTION"
        value = tostring(var.enable_fault_injection)
      }
      env {
        name        = "AUTH_TOKEN"
        secret_name = "auth-token"
      }

      liveness_probe {
        transport               = "HTTP"
        port                    = var.port
        path                    = "/healthz"
        interval_seconds        = 10
        failure_count_threshold = 3
      }

      readiness_probe {
        transport               = "HTTP"
        port                    = var.port
        path                    = "/healthz"
        interval_seconds        = 5
        failure_count_threshold = 3
      }
    }
  }

  ingress {
    external_enabled = true
    target_port      = var.port
    transport        = "http"

    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }
}
