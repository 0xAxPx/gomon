# GoMon Monitoring Platform - Infrastructure Design Document

**Project**: [gomon](https://github.com/0xAxPx/gomon)  
**Version**: 1.0  
**Last Updated**: August 30, 2025  
**Environment**: Development/Testing  

---

## Architecture Overview

The GoMon platform implements a comprehensive monitoring solution using a hybrid architecture combining Kubernetes orchestration with external services for optimal resource utilization and flexibility.

### High-Level Architecture

```
macOS Host Environment
├── Docker Desktop Kubernetes Cluster
│   ├── Monitoring Namespace (10 pods)
│   ├── Ingress Controller (nginx)
│   └── Persistent Storage (PVC-backed)
└── UTM Virtual Machine (192.168.0.45)
    └── External Elasticsearch (8.7.0)
```

---

## Kubernetes Cluster Configuration

### Cluster Details
- **Platform**: Docker Desktop on macOS
- **Version**: Kubernetes v1.28+
- **Namespaces**: monitoring (primary), ingress-nginx, argocd
- **Storage**: Local PVC with dynamic provisioning
- **Networking**: CNI with nginx-ingress controller

### Resource Allocation Summary
```
Total Cluster Resources:
├── Memory: ~10-15Gi allocated across all pods
├── CPU: ~4-6 cores allocated
├── Storage: 29Gi PVC allocation
└── Network: nginx-ingress with external IP (localhost)
```

---

## Pod Inventory and Configuration

### 1. Application Components

#### Agent (Metrics Collection)
```yaml
Pod: agent-fc997945c-bn45w
├── Image: ragazzo271985/agent:latest
├── Resources: 256Mi RAM, 200m CPU
├── Function: System metrics collection and Kafka publishing
├── Network: Internal service communication
└── Status: Running (6+ days uptime)
```

#### Aggregator (Metrics Processing)
```yaml
Pod: aggregator-74df69d946-78jzs
├── Containers: 2 (aggregator + filebeat sidecar)
├── Main Container:
│   ├── Image: ragazzo271985/aggregator:20250812-87e3e2a
│   ├── Resources: 512Mi RAM, 200m CPU
│   ├── Function: Kafka consumer → VictoriaMetrics publisher
│   └── Environment: Kafka brokers, VictoriaMetrics URL
├── Sidecar Container:
│   ├── Image: docker.elastic.co/beats/filebeat:8.11.0
│   ├── Resources: 256Mi RAM, 200m CPU
│   ├── Function: Log collection from /var/log/aggregator.log
│   └── Output: Logstash via beats protocol (port 5044)
├── Shared Volume: /var/log (emptyDir)
└── Status: Running (25+ hours uptime)
```

### 2. Message Queue Infrastructure

#### Kafka Cluster (3 nodes)
```yaml
StatefulSet: kafka-{0,1,2}
├── Image: confluentinc/cp-kafka:latest
├── Resources: 1Gi RAM, 500m CPU per pod
├── Storage: 10Gi PVC per replica
├── Configuration:
│   ├── Cluster: 3-node quorum
│   ├── Topics: metrics-v4 (primary)
│   ├── Replication: 3x for high availability
│   └── Network: Internal service discovery
├── Service Discovery:
│   ├── kafka-0.kafka.monitoring.svc.cluster.local:9092
│   ├── kafka-1.kafka.monitoring.svc.cluster.local:9092
│   └── kafka-2.kafka.monitoring.svc.cluster.local:9092
└── Status: Running (10+ days uptime)
```

### 3. Metrics Storage and Visualization

#### VictoriaMetrics (Time-Series Database)
```yaml
Pod: victoria-metrics-77b84f7c56-42qpb
├── Image: victoriametrics/victoria-metrics:latest
├── Resources: 1Gi RAM, 500m CPU
├── Storage: 20Gi PVC for metrics retention
├── Function: Time-series metrics storage
├── API: http://victoria-metrics:8428/api/v1/import
└── Status: Running (71+ days uptime)
```

#### Grafana (Metrics Visualization)
```yaml
Pod: grafana-7597f858d-fp4xv
├── Image: grafana/grafana:latest
├── Resources: 512Mi RAM, 300m CPU
├── Storage: 2Gi PVC for dashboards and configuration
├── Function: Metrics visualization and dashboarding
├── Access: http://grafana.local (via ingress)
└── Status: Running (91+ days uptime)
```

### 4. Log Processing Pipeline

#### Logstash (Log Processing)
```yaml
Pod: logstash-58474d4db-rzmmk
├── Image: docker.elastic.co/logstash/logstash:8.11.0
├── Resources: 1-2Gi RAM, 300-500m CPU
├── Configuration:
│   ├── Input: Beats protocol (port 5044)
│   ├── Filter: Ruby-based JSON parsing for VictoriaMetrics data
│   ├── Output: External Elasticsearch + stdout debugging
│   └── Features: Advanced message parsing, field extraction
├── Processing Capabilities:
│   ├── JSON extraction from log messages
│   ├── Structured field creation (metric_name, metric_value)
│   ├── Tag-based categorization
│   └── Real-time log transformation
└── Status: Running (24+ hours uptime)
```

#### Kibana (Log Visualization)
```yaml
Pod: kibana-57fcc4d755-5mnwh
├── Image: docker.elastic.co/kibana/kibana:8.7.0
├── Resources: 1Gi RAM, 500m CPU
├── Configuration:
│   ├── Elasticsearch: http://192.168.0.45:9200 (external)
│   ├── Index Patterns: aggregator* with 6 metric types
│   └── Dashboards: Real-time system metrics visualization
├── Features:
│   ├── Advanced index lifecycle management (ILM) support
│   ├── Real-time metrics dashboards (CPU, memory, disk, network)
│   ├── Log correlation and analysis tools
│   └── Time-series visualization for 11,500+ documents
├── Access: http://kibana.local (via ingress)
└── Status: Running (4+ days uptime)
```

### 5. Code Quality Platform

#### SonarQube (Static Code Analysis)
```yaml
Pod: sonarqube-xxx
├── Image: sonarqube:10.3-community
├── Resources: 2-4Gi RAM, 500m-1000m CPU
├── Storage: 12Gi total (10Gi data + 2Gi extensions)
├── Configuration:
│   ├── Database: PostgreSQL connection via HikariCP
│   ├── Internal Elasticsearch: Embedded for code search
│   ├── Go Language Support: Static analysis for agent/aggregator
│   └── Quality Gates: Custom "gomon-quality-gate"
├── Features:
│   ├── Clean as You Code methodology
│   ├── GitHub integration ready
│   ├── Zero tolerance for new vulnerabilities
│   └── 80% coverage requirement for new code
├── Access: http://sonarqube.local or localhost:9000
└── Status: Running and operational
```

#### PostgreSQL (SonarQube Database)
```yaml
Pod: postgres-5b74bc4cf6-mqpqr
├── Image: postgres:15
├── Resources: 512Mi-1Gi RAM, 200m-500m CPU
├── Storage: 5Gi PVC for database persistence
├── Configuration:
│   ├── Database: sonarqube
│   ├── User: sonarqube
│   ├── Connection Pool: HikariCP integration
│   └── Health Checks: pg_isready probes
├── Function: Persistent storage for SonarQube data
└── Status: Running and accepting connections
```

---

## Network Architecture

### Internal Kubernetes Networking
```
Service Discovery:
├── kafka-0.kafka.monitoring.svc.cluster.local:9092
├── kafka-1.kafka.monitoring.svc.cluster.local:9092  
├── kafka-2.kafka.monitoring.svc.cluster.local:9092
├── victoria-metrics.monitoring.svc.cluster.local:8428
├── logstash.monitoring.svc.cluster.local:5044
├── postgres.monitoring.svc.cluster.local:5432
└── sonarqube.monitoring.svc.cluster.local:9000
```

### Ingress Configuration
```yaml
nginx-ingress-controller:
├── External IP: localhost (Docker Desktop)
├── Ports: 80:30693/TCP, 443:31398/TCP
└── Routes:
    ├── grafana.local → grafana:3000
    ├── kibana.local → kibana:5601
    ├── victoria.local → victoria-metrics:8428
    └── sonarqube.local → sonarqube:9000
```

### External Integration
```
Hybrid Architecture:
├── Kubernetes Cluster (172.17.0.0/16 network)
├── External Elasticsearch: 192.168.0.45:9200 (UTM VM)
├── Connection Method: Direct IP routing
└── Stability: Enhanced with 2GB ES heap allocation
```

---

## Data Flow Architecture

### Metrics Pipeline
```
Agent → Kafka → Aggregator → VictoriaMetrics → Grafana
├── Collection: System metrics (CPU, memory, disk, network)
├── Transport: Kafka topics (metrics-v4)
├── Processing: Real-time aggregation and formatting
├── Storage: VictoriaMetrics time-series database
└── Visualization: Grafana dashboards
```

### Logging Pipeline
```
Aggregator App → /var/log/aggregator.log → Filebeat → Logstash → Elasticsearch → Kibana
├── Collection: Filebeat sidecar pattern
├── Processing: Logstash with Ruby-based JSON parsing
├── Parsing: Extract metric_name, metric_value from VictoriaMetrics JSON
├── Storage: External Elasticsearch with ILM policies
└── Visualization: Kibana with real-time dashboards
```

### Code Quality Pipeline
```
Go Source Code → SonarScanner → SonarQube → Quality Gates → PostgreSQL
├── Analysis: Static code analysis for agent/ and aggregator/
├── Quality Control: Custom "gomon-quality-gate" with Clean as You Code
├── Storage: PostgreSQL for project data, Internal ES for code search
└── Integration: GitHub-ready with PR quality checks
```

---

## Storage Configuration

### Persistent Volume Claims
```
Storage Allocation:
├── kafka-data-kafka-{0,1,2}: 10Gi each (30Gi total)
├── victoria-metrics-data: 20Gi
├── grafana-storage: 2Gi  
├── postgres-pvc: 5Gi
├── sonarqube-data-pvc: 10Gi
├── sonarqube-extensions-pvc: 2Gi
└── Total: 61Gi allocated
```

### Index Lifecycle Management (Elasticsearch)
```
ILM Policy: aggregator-rollover-policy
├── Hot Phase: Max 1 day or 2MB, priority 100
├── Warm Phase: 1+ days, force merge + shrink, priority 50
├── Cold Phase: 2+ days, storage optimization, priority 0
├── Delete Phase: 3+ days automatic cleanup
└── Current Indices: 4 active (aggregator-logs-000001 through 000004)
```

---

## Security and Access Control

### Authentication Methods
```
Service Access:
├── Grafana: admin/admin (local auth)
├── Kibana: No authentication (internal access)
├── SonarQube: admin/admin → custom password (token-based API)
├── ArgoCD: admin/[kubectl secret extraction]
└── Elasticsearch: No authentication (external VM)
```

### Network Security
```
Access Control:
├── Internal Services: ClusterIP (cluster-only access)
├── External Access: nginx-ingress with host-based routing
├── Port Forwarding: kubectl port-forward for development
└── Host Network: Available for Logstash if needed for VM connectivity
```

---

## Resource Monitoring and Observability

### Container Resource Control (cgroups v2)
```
Resource Limits Verification:
├── Aggregator: 512Mi memory limit, 200m CPU (20ms/100ms periods)
├── Filebeat: 256Mi memory limit, 200m CPU (40ms/200ms periods)
├── Logstash: 1-2Gi memory limit, 300-500m CPU
├── SonarQube: 2-4Gi memory limit, 500m-1000m CPU
└── Monitoring: /sys/fs/cgroup/ interface for resource tracking
```

### Performance Metrics
```
System Performance:
├── Log Processing: 11,500+ documents across 4 ILM-managed indices
├── Metrics Collection: 6 metric types (CPU, memory, disk, network)
├── Code Quality: PASSED quality gate with progressive improvement
├── Message Queue: 3-node Kafka cluster with high availability
└── Time-Series: VictoriaMetrics with optimized storage
```

---

## Network Troubleshooting and Stability

### Known Issues and Resolutions

#### Elasticsearch Connectivity (Resolved)
```
Issue: Intermittent connection failures to external ES
Root Cause: UTM VM resource constraints and ES heap pressure
Resolution: Increased ES heap from 1GB to 2GB (-Xmx2g)
Monitoring: Continuous connectivity verification from Kubernetes pods
```

#### Container Networking
```
Network Architecture:
├── Kubernetes Internal: 172.17.0.0/16 (Docker Desktop)
├── Pod Communication: ClusterIP services with DNS resolution
├── External Integration: Direct IP routing to UTM VM
└── Ingress Access: localhost routing via nginx-ingress
```

### Stability Enhancements
```
Reliability Measures:
├── Health Checks: Readiness/liveness probes for all critical pods
├── Resource Limits: cgroups v2 enforcement preventing resource conflicts
├── Persistent Storage: PVC-backed data retention across pod restarts
├── Connection Pooling: HikariCP for database connections
└── Retry Logic: Logstash retry mechanisms for external service failures
```

---

## Configuration Management

### GitOps Integration
```
ArgoCD Configuration:
├── Repository: https://github.com/0xAxPx/gomon
├── Target: monitoring namespace
├── Sync Policy: Manual with auto-sync capabilities
└── Management: Kubernetes manifest deployment and updates
```

### Key Configuration Files
```
Configuration Structure:
├── k8s/
│   ├── monitoring/
│   │   ├── agent-deployment.yaml
│   │   ├── aggregator-deployment.yaml (with filebeat sidecar)
│   │   ├── kafka-statefulset.yaml
│   │   ├── logstash-configmap.yaml (with Ruby JSON parsing)
│   │   └── victoria-metrics-deployment.yaml
│   ├── sonar/
│   │   ├── postgres-config.yaml
│   │   ├── sonarqube-deployment.yaml
│   │   └── sonarqube-ingress.yaml
│   └── ingress/
│       └── monitoring-ingress.yaml
├── sonar-project.properties (Go analysis configuration)
└── Quality Gates: gomon-quality-gate (custom rules)
```

---

## Data Processing and Analysis

### Log Processing Pipeline
```
Filebeat Configuration:
├── Input: /var/log/aggregator.log (sidecar pattern)
├── Fields: app, environment, service metadata
├── Output: logstash:5044 (beats protocol)
└── Processing: Real-time log streaming

Logstash Processing:
├── Input: beats {port => 5044}
├── Filter: Ruby-based JSON extraction from VictoriaMetrics messages
├── Parsing: metric_name, metric_value, timestamps extraction
├── Output: External Elasticsearch + stdout debugging
└── Performance: ~177 events processed, 8.7KB data transmitted

Elasticsearch Storage:
├── Location: External UTM VM (192.168.0.45:9200)
├── Version: 8.7.0 with 2GB heap allocation
├── ILM: 4-phase lifecycle management with automatic rollover
├── Indices: aggregator-logs-* with alias-based routing
└── Data Volume: 11,500+ documents across multiple indices
```

### Metrics Processing
```
Data Flow:
├── Agent: System metrics collection (CPU, memory, disk, network)
├── Kafka: Message queuing with metrics-v4 topic
├── Aggregator: JSON formatting for VictoriaMetrics API
├── VictoriaMetrics: Time-series storage and querying
└── Grafana: Visualization and alerting dashboards

Metric Types Processed:
├── cpu_usage_percent: 51 documents
├── mem_usage_percent: 51 documents  
├── disk_used_percent: 255 documents (most frequent)
├── dsk_used_gb: 51 documents
├── int_bytes_recv_mb: 51 documents
└── int_bytes_sent_mb: 51 documents
```

---

## Quality Assurance and Code Analysis

### SonarQube Configuration
```
Code Quality Platform:
├── Project: Agent-Aggregator-On-GO
├── Language: Go (agent/ and aggregator/ directories)
├── Analysis Scope: Complete codebase with unified metrics
├── Quality Gate: gomon-quality-gate (custom configuration)
└── Integration: GitHub-ready with Clean as You Code methodology

Quality Gate Rules:
├── Security: Zero tolerance for new vulnerabilities
├── Reliability: Zero tolerance for new bugs
├── Maintainability: Progressive improvement focus
├── Coverage: 80% requirement for new code
├── Duplication: ≤3% threshold for code reuse
└── Status: PASSED with progressive improvement (-1 issue trend)
```

### Development Workflow Integration
```
Code Quality Pipeline:
├── Local Analysis: SonarScanner with sonar-project.properties
├── Token Authentication: Secure API access for analysis
├── Quality Gates: Automated pass/fail criteria
├── GitHub Integration: Ready for PR automation
└── Continuous Improvement: Clean as You Code methodology
```

---

## Operational Procedures

### Health Monitoring
```bash
# Cluster Health Check
kubectl get pods -n monitoring
kubectl get pvc -n monitoring
kubectl get ingress -n monitoring

# Service Connectivity
kubectl exec deployment/logstash -n monitoring -- curl -I http://192.168.0.45:9200
kubectl port-forward svc/grafana 3000:3000 -n monitoring
kubectl port-forward svc/kibana 5601:5601 -n monitoring

# Resource Monitoring
kubectl exec deployment/aggregator -n monitoring -c aggregator -- cat /sys/fs/cgroup/memory.current
kubectl exec deployment/aggregator -n monitoring -c aggregator -- cat /sys/fs/cgroup/cpu.stat
```

### Maintenance Procedures
```bash
# Log Pipeline Restart
kubectl rollout restart deployment/logstash -n monitoring

# Kafka Cluster Management
kubectl get statefulset kafka -n monitoring
kubectl logs kafka-0 -n monitoring

# SonarQube Analysis
./sonar-scanner-4.8.0.2856-macosx/bin/sonar-scanner

# Database Health
kubectl exec deployment/postgres -n monitoring -- pg_isready -U sonarqube
```

---

## Performance Characteristics

### Throughput Metrics
```
Processing Capacity:
├── Log Processing: 11,500+ documents processed with real-time indexing
├── Metrics Collection: 6 metric types with 30-second intervals
├── Message Queue: High-availability Kafka with 3-node replication
├── Code Analysis: Complete Go codebase analysis in ~13 seconds
└── Search Performance: Sub-second query response in Kibana/Grafana
```

### Resource Efficiency
```
Optimization Results:
├── Container Density: 10 pods on single Docker Desktop cluster
├── Storage Efficiency: ILM automatic lifecycle management
├── Memory Management: cgroups v2 enforcement preventing resource conflicts
├── Network Optimization: Internal service discovery with minimal latency
└── Cost Efficiency: Community editions and open-source stack
```

---

## Technology Stack Summary

### Core Technologies
```
Container Orchestration:
├── Kubernetes: Docker Desktop with nginx-ingress
├── Storage: PVC with dynamic provisioning
├── Networking: CNI with service discovery
└── Security: RBAC and resource quotas

Monitoring and Observability:
├── Logs: ELK Stack (Filebeat → Logstash → Elasticsearch → Kibana)
├── Metrics: Agent → Kafka → Aggregator → VictoriaMetrics → Grafana
├── Code Quality: SonarQube → PostgreSQL with quality gates
└── Message Queue: Apache Kafka 3-node cluster

Languages and Frameworks:
├── Go: Agent and Aggregator microservices
├── YAML: Kubernetes manifests and configuration
├── Ruby: Logstash filter processing
└── JavaScript: Kibana and Grafana customizations
```

### Integration Capabilities
```
External Integrations:
├── GitHub: Repository integration with SonarQube quality gates
├── GitOps: ArgoCD for continuous deployment
├── Hybrid Architecture: K8s + VM integration patterns
└── Development Tools: SonarScanner with macOS compatibility
```

---

## Future Roadmap

### Phase 3: Infrastructure as Code (Next)
- Terraform implementation for AWS cloud services
- Infrastructure automation and provisioning
- Multi-environment deployment strategies
- Cost optimization and resource management

### Phase 4: Cloud Migration
- Amazon EKS evaluation and migration
- AWS managed services integration (OpenSearch, MSK)
- Production scalability and reliability
- Enterprise security and compliance

---

## Conclusion

The GoMon platform represents a comprehensive, production-ready monitoring solution with advanced observability, code quality assurance, and container orchestration capabilities. The hybrid architecture successfully balances resource efficiency with functionality, providing a solid foundation for cloud migration and enterprise scaling.

**Key Success Metrics:**
- **10 operational pods** with high availability
- **11,500+ log documents** processed with real-time analysis
- **PASSED quality gates** with progressive code improvement
- **Complete observability** across metrics, logs, and code quality
- **Production-ready architecture** with proper resource management and networking

**Platform Status**: Fully operational and ready for cloud infrastructure automation with Terraform.