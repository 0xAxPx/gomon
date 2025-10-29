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

locals {
  namespace = "monitoring"
  common_labels = {
    managed-by  = "terraform"
    environment = "development"
    project     = "gomon"
  }
}

# Create the monitoring namespace
resource "kubernetes_namespace" "monitoring" {
  metadata {
    name = local.namespace
    labels = merge(local.common_labels, {
      component = "namespace"
      tier      = "foundation"
    })
  }
}

# Create VictoriaMetrics PVC
resource "kubernetes_persistent_volume_claim" "victoria_metrics_data" {
  metadata {
    name      = "victoria-metrics-data"
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "victoria-metrics"
      tier      = "storage"
    })
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
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "victoria-metrics"
      tier      = "database"
    })
  }

  spec {
    replicas = 1

    strategy {
      type = "Recreate"
    }

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
          
          args = ["-storageDataPath=/var/lib/victoria-metrics", "-promscrape.config=/etc/vm/scrape.yml"]
          
          volume_mount {
            name       = "vm-data"
            mount_path = "/var/lib/victoria-metrics"
          }

          volume_mount {
            name       = "scrape-config"
            mount_path = "/etc/vm"
            read_only  = true
          }
          
          resources {
            requests = {
              cpu    = "500m"
              memory = "256Mi"
            }
            limits = {
              cpu    = "1000m"
              memory = "256Mi"
            }
          }
        }
        
        volume {
          name = "vm-data"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.victoria_metrics_data.metadata[0].name
          }
        }
        volume {
          name = "scrape-config"
          config_map {
            name = kubernetes_config_map.victoria_metrics_scrape_config.metadata[0].name
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
    namespace = local.namespace
    labels = merge(local.common_labels, {
    component = "victoria-metrics"
    tier      = "networking"
  })
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

# VictoriaMetrics Scrape Configuration
resource "kubernetes_config_map" "victoria_metrics_scrape_config" {
  metadata {
    name      = "victoria-metrics-scrape-config"
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "victoria-metrics"
      tier      = "configuration"
    })
  }

  data = {
    "scrape.yml" = <<-EOF
      global:
        scrape_interval: 30s
        scrape_timeout: 10s
        external_labels:
          cluster: 'gomon-dev'
          environment: 'development'
      
      scrape_configs:
      # Alerting Service Metrics
      - job_name: 'alerting-service'
        static_configs:
        - targets: ['alerting.monitoring.svc.cluster.local:8099']
          labels:
            service: 'alerting'
            component: 'alerting-service'
        metrics_path: '/metrics'
        scrape_interval: 30s
        scrape_timeout: 10s
      
      # VictoriaMetrics Self-Monitoring
      - job_name: 'victoria-metrics'
        static_configs:
        - targets: ['localhost:8428']
          labels:
            service: 'victoria-metrics'
        metrics_path: '/metrics'
        scrape_interval: 30s
    EOF
  }
}

# Postgres
resource "kubernetes_config_map" "postgres_config" {
  metadata {
    name      = "postgres-config"
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "postgresql"
      tier      = "storage"
    })
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
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "postgresql"
      tier      = "storage"
    })
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
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "postgresql"
      tier      = "database"
    })
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
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "postgresql"
      tier      = "networking"
    })
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
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "ingress"
      tier      = "networking"
    })
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

    rule {
      host = "jaeger.local"

      http {
        path {
          path = "/"
          path_type = "Prefix"

          backend {
            service {
              name = "jaeger"
              port {
                number = 16686
              }
            }
          }
        }
      }
    }
    rule {
      host = "alerting.local"

      http {
        path {
          path = "/"
          path_type = "Prefix"

          backend {
            service {
              name = "alerting"
              port {
                number = 8099
              }
            }
          }
        }
      }
    }  
  }
}


# ES Config Map (Nginx)
resource "kubernetes_config_map" "elasticsearch_lb_config" {
  metadata {
    name = "elasticsearch-lb-config"
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "elasticsearch-loadbalancer"
      tier      = "networking"
    })
  }
data = {
  "nginx.conf" = <<-EOF
events {
        worker_connections 1024;
    }
    
    http {
        # ✅ ROBUST: Extended timeouts for external VMs
        proxy_connect_timeout       45s;
        proxy_send_timeout          90s;
        proxy_read_timeout          300s;
        
        # ✅ CONNECTION POOLING: Optimized for external connections
        upstream elasticsearch_cluster {
            # Primary ES node - EXTERNAL VM
            server 192.168.0.45:9200 max_fails=3 fail_timeout=30s weight=2;
            # Secondary ES node - EXTERNAL VM  
            server 192.168.0.157:9200 max_fails=3 fail_timeout=30s weight=1;
            
            # External connection optimization
            keepalive 4;
            keepalive_requests 1000;
            keepalive_timeout 300s;
        }
        
        # ✅ HEALTH CHECK: LB internal health
        server {
            listen 8080;
            
            location /health {
                access_log off;
                return 200 "nginx-lb-operational\n";
                add_header Content-Type text/plain;
            }
            
            # ✅ EXTERNAL HEALTH: Proxy ES cluster health
            location /es-health {
                proxy_pass http://elasticsearch_cluster/_cluster/health;
                proxy_connect_timeout 10s;
                proxy_read_timeout 30s;
                access_log off;
            }
            
            # ✅ DEBUGGING: Individual node health checks
            location /es1-health {
                proxy_pass http://192.168.0.45:9200/_cluster/health;
                proxy_connect_timeout 5s;
                proxy_read_timeout 10s;
                access_log off;
            }
            
            location /es2-health {
                proxy_pass http://192.168.0.157:9200/_cluster/health;
                proxy_connect_timeout 5s;
                proxy_read_timeout 10s;
                access_log off;
            }
        }
        
        # ✅ MAIN PROXY: Elasticsearch API
        server {
            listen 9200;
            
            # Buffer settings for large ES responses
            proxy_buffer_size 128k;
            proxy_buffers 8 256k;
            proxy_busy_buffers_size 512k;
            proxy_temp_file_write_size 512k;
            
            # ✅ ALL TRAFFIC: Route to ES cluster
            location / {
                proxy_pass http://elasticsearch_cluster;
                
                # Essential headers
                proxy_set_header Host $host;
                proxy_set_header X-Real-IP $remote_addr;
                proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
                proxy_set_header X-Forwarded-Proto $scheme;
                proxy_set_header Connection "";
                
                # ✅ EXTERNAL VM TIMEOUTS: Generous for network latency
                proxy_connect_timeout 45s;
                proxy_send_timeout 90s;
                proxy_read_timeout 600s;    # ES searches can be slow
                
                # ✅ FAILOVER: Automatic retry on external VM failure
                proxy_next_upstream error timeout http_502 http_503 http_504;
                proxy_next_upstream_tries 3;
                proxy_next_upstream_timeout 60s;
                
                # Large request handling
                client_max_body_size 100m;
                proxy_request_buffering off;
                
                # Add debugging headers
                add_header X-Upstream-Server $upstream_addr always;
                add_header X-Response-Time $upstream_response_time always;
            }
        }
        
        # Logging
        error_log /var/log/nginx/error.log info;
        access_log /var/log/nginx/access.log;
    }
  EOF
}
}
# ES Deployment
resource "kubernetes_deployment" "elasticsearch_lb" {
  metadata {
    name = "elasticsearch-lb"
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "elasticsearch-loadbalancer"
      tier      = "networking"
    })
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "elasticsearch-lb"
      }
    }

    template {
      metadata {
        labels = {
          app = "elasticsearch-lb"
        }
      }

      spec {
        host_network = true
        dns_policy = "ClusterFirstWithHostNet"

        container {
          name = "nginx"
          image = "nginx:1.25-alpine"

          port {
            container_port = 9200
            name = "elasticsearch"
            protocol = "TCP"
          }

          port {
            container_port = 8080
            name = "health"
            protocol = "TCP"
          }

          volume_mount {
            name = "nginx-config"
            mount_path = "/etc/nginx/nginx.conf"
            sub_path = "nginx.conf"
          }

          volume_mount {
            name = "nginx-logs"
            mount_path = "/var/log/nginx"
          }

          resources {
            requests = {
              cpu = "100m"
              memory = "64Mi"
            }
            limits = {
              cpu = "300m"
              memory = "64Mi"
            }
          }

          readiness_probe {
            http_get {
              path = "/health"
              port = 8080
              host = "localhost"
            }
            initial_delay_seconds = 15
            period_seconds = 10
            timeout_seconds = 5
            failure_threshold = 3
          }

          liveness_probe {
            http_get {
              path = "/health"
              port = 8080
              host = "localhost"
            }
            initial_delay_seconds = 30
            period_seconds = 30
            timeout_seconds = 10
            failure_threshold = 5
          }

        }
        init_container {
          name = "connectivity-test"
          image = "curlimages/curl:8.4.0"

          command = ["/bin/sh", "-c"]
          args = [<<EOF
                echo "🔍 Testing external ES connectivity from HOST NETWORK..."
                echo "Testing Node 1 (192.168.0.45:9200):"
                if curl -m 15 -f http://192.168.0.45:9200; then
                  echo "✅ Node 1 reachable"
                else
                  echo "❌ Node 1 FAILED - Exit code: $?"
                  exit 1
                fi

                echo "Testing Node 2 (192.168.0.157:9200):"
                if curl -m 15 -f http://192.168.0.157:9200; then
                  echo "✅ Node 2 reachable"
                else
                  echo "❌ Node 2 FAILED - Exit code: $?"
                exit 1
                fi

                echo "🎉 All ES nodes accessible from host network!"
                EOF 
                ]
        }

        volume {
          name = "nginx-config"
          config_map {
            name = kubernetes_config_map.elasticsearch_lb_config.metadata[0].name
          }
        }

        volume {
          name = "nginx-logs"
          empty_dir {
            
          }
        }
      }
    }
  }
}

#ES Load Balancer Service
resource "kubernetes_service" "elasticsearch_lb" {
  metadata {
    name      = "elasticsearch-lb"
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "elasticsearch-loadbalancer"
      tier      = "networking"
    })
  }

  spec {
    type = "ClusterIP"
    
    selector = {
      app = "elasticsearch-lb"
    }
    
    port {
      port        = 9200
      target_port = 9200
      name        = "elasticsearch"
      protocol    = "TCP"
    }
    
    port {
      port        = 8080
      target_port = 8080
      name        = "health"
      protocol    = "TCP"
    }
  }
}

# ES Load Balancer Service External
resource "kubernetes_service" "elasticsearch_lb_external" {
  metadata {
    name      = "elasticsearch-lb-external"
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "elasticsearch-loadbalancer"
      tier      = "networking"
    })
  }

  spec {    
    type = "NodePort"
    selector = {
      app = "elasticsearch-lb"
    }
    
    port {
      port        = 9200
      target_port = 9200
      node_port   = 30920
      name        = "elasticsearch"
    }
    
    port {
      port        = 8080
      target_port = 8080
      node_port   = 30921
      name        = "health"
    }
  }
}


# Jaeger Deployment
resource "kubernetes_deployment" "jaeger" {
  metadata {
    name      = "jaeger"
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "jaeger"
      tier      = "tracing"
    })
  }

  spec {
    replicas = 1
    selector {
      match_labels = {
        app = "jaeger"
      }
    }

    template {
      metadata {
        labels = {
          app = "jaeger"
        }
      }

      spec {
        container {
          name  = "jaeger"
          image = "jaegertracing/all-in-one:1.52"

          # Jaeger UI
          port {
            container_port = 16686
            name = "ui"
          }

          # OTLP receivers
          port {
            container_port = 4317
            name = "otlp-grpc"
          }

          port {
            container_port = 4318
            name = "otlp-http"
          }

          # Jaeger native protocols
          port {
            container_port = 14250
            name = "grpc"
          }

          port {
            container_port = 6832
            name = "agent-udp"
            protocol = "UDP"
          }

          port {
            container_port = 14268
            name = "agent-http"
          }

          env {
            name  = "COLLECTOR_OTLP_ENABLED"
            value = "true"
          }

          env {
            name  = "COLLECTOR_ZIPKIN_HTTP_PORT"
            value = "9411"
          }

          resources {
            requests = {
              cpu    = "100m"
              memory = "128Mi"
            }
            limits = {
              cpu    = "500m"
              memory = "128Mi"
            }
          }

          readiness_probe {
            http_get {
              path = "/"
              port = 16686
            }
            initial_delay_seconds = 30
            period_seconds        = 10
          }

          liveness_probe {
            http_get {
              path = "/"
              port = 16686
            }
            initial_delay_seconds = 60
            period_seconds        = 30
          }
        }
      }
    }
  }
}

# Jaeger Service
resource "kubernetes_service" "jaeger" {
  metadata {
    name      = "jaeger"
    namespace = local.namespace
    labels = merge(local.common_labels, {
      component = "jaeger"
      tier      = "networking"
    })
  }

  spec {
    type = "ClusterIP"
    
    selector = {
      app = "jaeger"
    }
    
    # Jaeger UI
    port {
      port        = 16686
      target_port = 16686
      name        = "ui"
      protocol    = "TCP"
    }

    # OTLP receivers
    port {
      port        = 4317
      target_port = 4317
      name        = "otlp-grpc"
      protocol    = "TCP"
    }

    port {
      port        = 4318
      target_port = 4318
      name        = "otlp-http"
      protocol    = "TCP"
    }

    # Jaeger native
    port {
      port        = 14250
      target_port = 14250
      name        = "grpc"
      protocol    = "TCP"
    }

    port {
      port        = 6832
      target_port = 6832
      name        = "agent-udp"
      protocol    = "UDP"
    }

    port {
      port        = 14268
      target_port = 14268
      name        = "agent-http"
      protocol    = "TCP"
    }
  }
}