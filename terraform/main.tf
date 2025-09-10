terraform {
  required_version = ">= 1.0"
  
  cloud {
    organization = "gomon"
    
    workspaces {
      name = "gomon-dev"
    }
  }
  
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.23"
    }
  }
}

provider "kubernetes" {
    config_path = "~/.kube/config"
}

# Create the monitoring namespace
resource "kubernetes_namespace" "monitoring" {
  metadata {
    name = "monitoring"
    labels = {
      name        = "monitoring"
      managed-by  = "terraform"
      environment = "development"
    }
  }
}

# Create VictoriaMetrics PVC
resource "kubernetes_persistent_volume_claim" "victoria_metrics_data" {
  metadata {
    name      = "victoria-metrics-data"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
    labels = {
      app        = "victoria-metrics"
      managed-by = "terraform"
    }
  }
  
  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        storage = "20Gi"
      }
    }
  }
}

# Create VictoriaMetrics Deployment
resource "kubernetes_deployment" "victoria_metrics" {
  metadata {
    name      = "victoria-metrics"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
    labels = {
      app        = "victoria-metrics"
      managed-by = "terraform"
    }
  }

  spec {
    replicas = 1
    selector {
      match_labels = {
        app = "victoria-metrics"
      }
    }

    template {
      metadata {
        labels = {
          app = "victoria-metrics"
        }
      }

      spec {
        container {
          name  = "victoria-metrics"
          image = "victoriametrics/victoria-metrics:v1.101.0"
          
          port {
            container_port = 8428
          }
          
          args = ["-storageDataPath=/var/lib/victoria-metrics"]
          
          volume_mount {
            name       = "vm-data"
            mount_path = "/var/lib/victoria-metrics"
          }
          
          resources {
            requests = {
              cpu    = "500m"
              memory = "1Gi"
            }
            limits = {
              cpu    = "1000m"
              memory = "2Gi"
            }
          }
        }
        
        volume {
          name = "vm-data"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.victoria_metrics_data.metadata[0].name
          }
        }
      }
    }
  }
}

# Create VictoriaMetrics Service
resource "kubernetes_service" "victoria_metrics" {
  metadata {
    name      = "victoria-metrics"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
    labels = {
      app        = "victoria-metrics"
      managed-by = "terraform"
    }
  }

  spec {
    type = "ClusterIP"
    
    port {
      port        = 8428
      target_port = 8428
      protocol    = "TCP"
    }
    
    selector = {
      app = "victoria-metrics"
    }
  }
}