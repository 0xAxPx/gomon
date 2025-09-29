# GoMon - Kubernetes Monitoring Platform

A comprehensive, production-ready monitoring and observability platform built on Kubernetes, featuring real-time metrics collection, distributed tracing, log aggregation, and intelligent alerting.

[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.28+-326CE5?logo=kubernetes&logoColor=white)](https://kubernetes.io/)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-Multi--stage-2496ED?logo=docker&logoColor=white)](https://www.docker.com/)

---

## 🏗️ Architecture Overview

GoMon implements a microservices-based monitoring solution with three core components orchestrated on Kubernetes:

```
┌─────────────────────────────────────────────────────────────────┐
│                      Kubernetes Cluster                          │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    Monitoring Namespace                      │ │
│  │                                                              │ │
│  │  ┌──────────┐    ┌───────────────┐    ┌──────────────┐   │ │
│  │  │  Agent   │───▶│  Kafka (3x)   │───▶│  Aggregator  │   │ │
│  │  │          │    │               │    │              │   │ │
│  │  └──────────┘    └───────────────┘    └──────────────┘   │ │
│  │       │                                      │             │ │
│  │       │                                      ▼             │ │
│  │       │                         ┌────────────────────────┐│ │
│  │       │                         │  VictoriaMetrics       ││ │
│  │       │                         └────────────────────────┘│ │
│  │       │                                      │             │ │
│  │       │                                      ▼             │ │
│  │       │                         ┌────────────────────────┐│ │
│  │       │                         │      Grafana           ││ │
│  │       │                         └────────────────────────┘│ │
│  │       │                                                    │ │
│  │       ▼                                                    │ │
│  │  ┌──────────────┐        ┌────────────────┐              │ │
│  │  │   Jaeger     │        │   PostgreSQL   │              │ │
│  │  │  (Tracing)   │        │                │              │ │
│  │  └──────────────┘        └────────────────┘              │ │
│  │                                  │                        │ │
│  │       │                          ▼                        │ │
│  │       │                  ┌──────────────┐                │ │
│  │       │                  │  Alerting    │                │ │
│  │       │                  │   Service    │                │ │
│  │       │                  └──────────────┘                │ │
│  │       │                                                   │ │
│  │       ▼                                                   │ │
│  │  ┌────────────┐       ┌──────────┐      ┌──────────┐   │ │
│  │  │ Filebeat   │──────▶│ Logstash │─────▶│   ELK    │   │ │
│  │  │            │       │          │      │ (VM)     │   │ │
│  │  └────────────┘       └──────────┘      └──────────┘   │ │
│  │                                                          │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │               Ingress (nginx) - Local Access               │ │
│  │  grafana.local | kibana.local | jaeger.local              │ │
│  │  sonarqube.local | alerting.local                         │ │
│  └──────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

## 🎯 Core Components

### **1. Agent** - Metrics Collection
**Language**: Go | **Image**: `ragazzo271985/agent:latest`

Lightweight system metrics collector that gathers CPU, memory, disk, and network statistics.

- **Features**:
  - Real-time system metrics collection (gopsutil)
  - Protobuf serialization for efficient transport
  - Jaeger distributed tracing integration
  - Concurrent metric gathering with goroutines
  - Kafka producer with automatic retry

- **Deployment**: 
  - Resources: 256Mi RAM, 200m CPU
  - Collection interval: 20 seconds
  - Output: Kafka topic `metrics-topic`

### **2. Aggregator** - Data Processing
**Language**: Go | **Image**: `ragazzo271985/aggregator:latest`

Consumes metrics from Kafka, processes them, and publishes to VictoriaMetrics for long-term storage.

- **Features**:
  - High-throughput Kafka consumer
  - Protobuf deserialization
  - VictoriaMetrics remote write protocol
  - Filebeat sidecar for application logs
  - Concurrent processing with worker pools

- **Deployment**:
  - Multi-container pod (aggregator + filebeat)
  - Resources: 512Mi RAM, 200m CPU
  - Log output: `/var/log/aggregator.log` → Logstash

### **3. Alerting Service** - Incident Management
**Language**: Go | **Image**: `ragazzo271985/alerting-service:latest`

Intelligent alerting and incident management system with PostgreSQL backend.

- **Features**:
  - RESTful API for alert management
  - PostgreSQL with JSONB for flexible schemas
  - Kubernetes health monitoring (planned)
  - Grafana webhook integration (planned)
  - Slack bot integration (planned)

- **Deployment**:
  - Resources: 256Mi RAM, 100m CPU
  - Health endpoint: `/health/database`
  - External access: `http://alerting.local`

---

## 📊 Observability Stack

### **Metrics Pipeline**
```
Agent → Kafka → Aggregator → VictoriaMetrics → Grafana
```
- **VictoriaMetrics**: Time-series database optimized for metrics
- **Grafana**: Visualization and dashboards (`http://grafana.local`)

### **Logging Pipeline**
```
Application → Filebeat → Logstash → Elasticsearch → Kibana
```
- **Elasticsearch**: 8.7.0 running on external VM (192.168.0.45)
- **Logstash**: Filter and enrichment with Ruby processing
- **Kibana**: Log search and analysis (`http://kibana.local`)

### **Distributed Tracing**
```
Agent → Jaeger Collector → Jaeger Query → UI
```
- **Jaeger**: OpenTracing-compatible distributed tracing
- **UI**: Trace visualization (`http://jaeger.local`)

### **Code Quality**
```
GitHub → SonarScanner → SonarQube → PostgreSQL
```
- **SonarQube**: Static analysis and quality gates
- **UI**: Code quality dashboard (`http://sonarqube.local`)

---

## 🛠️ Technology Stack

### **Core Technologies**
- **Container Orchestration**: Kubernetes (Docker Desktop)
- **Service Mesh**: Native K8s service discovery
- **Ingress**: nginx-ingress-controller
- **Storage**: PVC with dynamic provisioning (29Gi allocated)

### **Languages & Frameworks**
- **Go 1.22**: Agent, Aggregator, Alerting microservices
- **Protocol Buffers**: Efficient data serialization
- **Gin**: HTTP framework for REST APIs

### **Data Infrastructure**
- **Apache Kafka**: 3-node cluster for message streaming
- **PostgreSQL 15**: Relational database (SonarQube, Alerting)
- **VictoriaMetrics**: Metrics storage
- **Elasticsearch 8.7**: Log storage (external VM)

### **Observability Tools**
- **Grafana**: Metrics visualization
- **Kibana**: Log exploration
- **Jaeger**: Distributed tracing
- **SonarQube**: Code quality analysis

### **GitOps & CI/CD**
- **ArgoCD**: Continuous deployment
- **GitHub**: Source control
- **Docker Hub**: Container registry (`ragazzo271985/*`)

---

## 🌐 Networking

### **Internal Service Communication**
All services communicate via Kubernetes internal DNS:
```
service-name.namespace.svc.cluster.local
```

Example connections:
- Agent → `kafka-0.monitoring.svc.cluster.local:9092`
- Aggregator → `victoria-metrics.monitoring.svc.cluster.local:8428`
- Alerting → `postgres.monitoring.svc.cluster.local:5432`

### **External Access via Ingress**
Services exposed through nginx-ingress on `localhost`:

| Service | URL | Purpose |
|---------|-----|---------|
| Grafana | http://grafana.local | Metrics dashboards |
| Kibana | http://kibana.local | Log analysis |
| Jaeger | http://jaeger.local | Trace visualization |
| SonarQube | http://sonarqube.local | Code quality |
| Alerting | http://alerting.local | Alert API |

**Setup**: Add to `/etc/hosts`:
```bash
127.0.0.1 grafana.local kibana.local jaeger.local sonarqube.local alerting.local
```

---

## 🚀 Getting Started

### **Prerequisites**
- Docker Desktop with Kubernetes enabled
- kubectl configured for local cluster
- 16GB+ RAM recommended
- macOS, Linux, or Windows with WSL2

### **Quick Start**

1. **Clone the repository**
```bash
git clone https://github.com/0xAxPx/gomon.git
cd gomon
```

2. **Deploy infrastructure**
```bash
# Apply Kubernetes manifests
kubectl apply -f k8s/

# Verify pods are running
kubectl get pods -n monitoring
```

3. **Configure local access**
```bash
# Add hostnames to /etc/hosts
echo "127.0.0.1 grafana.local kibana.local jaeger.local sonarqube.local alerting.local" | sudo tee -a /etc/hosts
```

4. **Access dashboards**
- Grafana: http://grafana.local
- Kibana: http://kibana.local  
- Jaeger: http://jaeger.local
- Alerting Health: http://alerting.local/health/database

### **Building Custom Images**

```bash
# Build agent
docker build -t ragazzo271985/agent:latest -f Dockerfile.agent .
docker push ragazzo271985/agent:latest

# Build aggregator
docker build -t ragazzo271985/aggregator:latest -f Dockerfile.aggregator .
docker push ragazzo271985/aggregator:latest

# Build alerting service
docker build -t ragazzo271985/alerting-service:latest -f alerting/Dockerfile.alerting .
docker push ragazzo271985/alerting-service:latest
```

---

## 📁 Project Structure

```
gomon/
├── agent/                    # Metrics collection service
│   └── main.go
├── aggregator/               # Data processing service
│   └── aggregator.go
├── alerting/                 # Alerting service
│   ├── cmd/alerter/
│   ├── internal/
│   ├── configs/
│   └── k8s/
├── kafka/                    # Kafka producer/consumer
├── pb/                       # Protocol Buffer definitions
├── k8s/                      # Kubernetes manifests
│   ├── agent/
│   ├── aggregator/
│   ├── kafka/
│   ├── victoria-metrics/
│   ├── postgres/
│   └── ...
├── terraform/                # Infrastructure as Code (planned)
├── docs/                     # Documentation
│   ├── gomon-alerting-design.md
│   └── gomon_infrastructure_design.md
├── Dockerfile.agent
├── Dockerfile.aggregator
└── go.mod
```

---

## 📈 Performance Metrics

### **Current Capacity**
- **Metrics Throughput**: ~3 metrics/second per agent
- **Log Processing**: 11,500+ documents indexed
- **Pod Density**: 13+ pods on single Docker Desktop cluster
- **Resource Efficiency**: ~10-15Gi memory, 4-6 CPU cores

### **Storage**
- **PVC Allocation**: 29Gi total
- **Elasticsearch**: External VM (192.168.0.45)
- **VictoriaMetrics**: Time-series compression
- **PostgreSQL**: ACID-compliant relational storage

---

## 🔐 Security Features

- **Non-root containers**: All services run as unprivileged users
- **RBAC**: Kubernetes role-based access control
- **Resource limits**: CPU and memory quotas enforced
- **Network policies**: Service-to-service restrictions
- **TLS**: CA certificates for HTTPS communications

---

## 🔧 Configuration

### **Environment Variables**

**Agent:**
```yaml
KAFKA_BROKERS: kafka-0:9092,kafka-1:9092,kafka-2:9092
KAFKA_TOPIC: metrics-topic
JAEGER_AGENT_HOST: jaeger
JAEGER_AGENT_PORT: 6831
```

**Aggregator:**
```yaml
KAFKA_BROKERS: kafka-0:9092,kafka-1:9092,kafka-2:9092
KAFKA_TOPIC: metrics-topic
VICTORIA_METRICS_URL: http://victoria-metrics:8428/api/v1/write
```

**Alerting:**
```yaml
CONFIG_PATH: /app/configs/prod.yaml
GIN_MODE: release
```

---

## 📊 Monitoring the Monitors

The platform includes self-monitoring capabilities:

- **Kafka**: JMX metrics exported to Prometheus
- **PostgreSQL**: Query performance and connection pooling
- **Kubernetes**: Metrics-server for resource tracking
- **Application**: Structured logging to Elasticsearch

---

## 🗺️ Roadmap

### **Phase 3: Infrastructure as Code** (Current)
- ✅ Alerting service deployment
- 🔄 Terraform for Kubernetes resources
- 📋 Alert Management API
- 📋 Kubernetes pod monitoring

### **Phase 4: Cloud Migration** (Planned)
- AWS EKS deployment
- Managed services (MSK, RDS, OpenSearch)
- Auto-scaling and high availability
- Multi-region support

### **Phase 5: Advanced Features** (Future)
- Machine learning anomaly detection
- Predictive alerting
- Self-healing automation
- Multi-cloud support

---

## 🤝 Contributing

Contributions are welcome! This is a learning-focused project.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📝 License

This project is open-source and available for educational purposes.

---

## 👤 Author

**GitHub**: [@0xAxPx](https://github.com/0xAxPx)

---

## 🙏 Acknowledgments

Built with:
- Kubernetes & Docker ecosystem
- Elastic Stack (ELK)
- CNCF projects (Jaeger, Grafana)
- Go community libraries
- Open-source monitoring tools

---

**Project Status**: ✅ Production-ready for local development  
**Last Updated**: September 29, 2025