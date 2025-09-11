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

resource "kubernetes_config_map" "postgres_config" {
  metadata {
    name      = "postgres-config"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
    labels = {
      app        = "postgres"
      managed-by = "terraform"
    }
  }

  data = {
    POSTGRES_DB       = "sonarqube"
    POSTGRES_USER     = "sonarqube"
    POSTGRES_PASSWORD = "sonarqube123"
  }
}

resource "kubernetes_persistent_volume_claim" "postgres_pvc" {
  metadata {
    name      = "postgres-pvc"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
    labels = {
      app        = "postgres"
      managed-by = "terraform"
    }
  }
  
  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        storage = "5Gi"
      }
    }
  }
}

resource "kubernetes_deployment" "postgres" {
  metadata {
    name      = "postgres"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
    labels = {
      app        = "postgres"
      managed-by = "terraform"
    }
  }

  spec {
    replicas = 1
    selector {
      match_labels = {
        app = "postgres"
      }
    }

    template {
      metadata {
        labels = {
          app = "postgres"
        }
      }

      spec {
        container {
          name  = "postgres"
          image = "postgres:15"
          
          port {
            container_port = 5432
          }
          
          env_from {
            config_map_ref {
              name = kubernetes_config_map.postgres_config.metadata[0].name
            }
          }
          
          volume_mount {
            name       = "postgres-storage"
            mount_path = "/var/lib/postgresql/data"
          }
          
          resources {
            requests = {
              cpu    = "200m"
              memory = "512Mi"
            }
            limits = {
              cpu    = "500m"
              memory = "1Gi"
            }
          }

          readiness_probe {
            exec {
              command = ["pg_isready", "-U", "sonarqube"]
            }
            initial_delay_seconds = 30
            period_seconds        = 10
          }

          liveness_probe {
            exec {
              command = ["pg_isready", "-U", "sonarqube"]
            }
            initial_delay_seconds = 60
            period_seconds        = 30
          }
        }
        
        volume {
          name = "postgres-storage"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.postgres_pvc.metadata[0].name
          }
        }
      }
    }
  }
}

# PostgreSQL Service
resource "kubernetes_service" "postgres" {
  metadata {
    name      = "postgres"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
    labels = {
      app        = "postgres"
      managed-by = "terraform"
    }
  }

  spec {
    type = "ClusterIP"
    
    port {
      port        = 5432
      target_port = 5432
      protocol    = "TCP"
    }
    
    selector = {
      app = "postgres"
    }
  }
}

# Create Ingress Resourse
resource "kubernetes_ingress_v1" "monitoring_ingress" {
  metadata {
    name      = "monitoring-ingress"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
    labels = {
      app        = "monitoring-ingress"
      managed-by = "terraform"
    }
    annotations = {
      "nginx.ingress.kubernetes.io/rewrite-target"      = "/"
      "nginx.ingress.kubernetes.io/proxy-buffer-size"   = "16k"
      "nginx.ingress.kubernetes.io/proxy-read-timeout"  = "600"
      "nginx.ingress.kubernetes.io/proxy-send-timeout"  = "600"
    }
  }

  spec {
    ingress_class_name = "nginx"
    
    rule {
      host = "grafana.local"

      http {
        path {
          path = "/"
          path_type = "Prefix"

          backend {
            service {
              name = "grafana"
              port {
                number = 3000
              }
            }
          }

        }
      }
    }

    rule {
      host = "victoria.local"

      http {
        path {
          path = "/"
          path_type = "Prefix"

          backend {
            service {
              name = "victoria-metrics"
              port {
                number = 8428
              }
            }
          }
        }
      }
    }
    
    rule {
      host = "kibana.local"

      http {
        path {
          path = "/"
          path_type = "Prefix"

          backend {
            service {
              name = "kibana"
              port {
                number = 5601
              }
            }
          }
        }

        path {
          path = "/spaces"
          path_type = "Prefix"

          backend {
            service {
              name = "kibana"
              port {
                number = 5601
              }
            }
          }
        }
      }
    }  
}
}
      