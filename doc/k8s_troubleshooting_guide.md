# Kubernetes Health Check and Troubleshooting Guide
## Network Connectivity and Storage Management

**Project**: GoMon Infrastructure  
**Environment**: Docker Desktop on macOS  
**Namespace**: monitoring  

---

## 1. Kubernetes Cluster Health Assessment

### Cluster-Level Health Checks

#### Node Status and Resources
```bash
# Check all nodes status and resources
kubectl get nodes -o wide

# Node resource capacity and allocation
kubectl describe nodes

# Node resource usage (requires metrics-server)
kubectl top nodes

# Check node conditions (Ready, MemoryPressure, DiskPressure)
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{": "}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}'
```

#### Cluster Component Status
```bash
# Check system pods
kubectl get pods -n kube-system

# Check ingress controller
kubectl get pods -n ingress-nginx

# Control plane components
kubectl get componentstatuses

# API server health
kubectl cluster-info
```

#### Resource Quotas and Limits
```bash
# Check namespace resource usage
kubectl describe namespace monitoring

# Resource quotas (if configured)
kubectl get resourcequota -n monitoring

# Limit ranges
kubectl get limitrange -n monitoring
```

### Pod-Level Health Assessment

#### Pod Status Overview
```bash
# All pods in monitoring namespace
kubectl get pods -n monitoring

# Pods with detailed status
kubectl get pods -n monitoring -o wide

# Pod events for troubleshooting
kubectl get events -n monitoring --sort-by='.lastTimestamp'

# Failed or problematic pods
kubectl get pods -n monitoring --field-selector=status.phase!=Running
```

#### Individual Pod Health
```bash
# Detailed pod information
kubectl describe pod <pod-name> -n monitoring

# Pod resource usage
kubectl top pod <pod-name> -n monitoring --containers

# Pod logs with context
kubectl logs <pod-name> -n monitoring --tail=50 --previous

# Multi-container pod logs (like aggregator)
kubectl logs <pod-name> -c <container-name> -n monitoring
```

#### Container Resource Monitoring
```bash
# Check container resource limits and usage
kubectl exec <pod-name> -n monitoring -- cat /sys/fs/cgroup/memory.max
kubectl exec <pod-name> -n monitoring -- cat /sys/fs/cgroup/memory.current
kubectl exec <pod-name> -n monitoring -- cat /sys/fs/cgroup/cpu.max

# Process information inside container
kubectl exec <pod-name> -n monitoring -- ps aux
kubectl exec <pod-name> -n monitoring -- free -h
kubectl exec <pod-name> -n monitoring -- df -h
```

---

## 2. Network Connectivity Troubleshooting

### Service Discovery and DNS

#### Internal DNS Resolution
```bash
# Test DNS resolution from pod
kubectl exec <pod-name> -n monitoring -- nslookup kubernetes.default
kubectl exec <pod-name> -n monitoring -- nslookup <service-name>.monitoring.svc.cluster.local

# DNS configuration check
kubectl exec <pod-name> -n monitoring -- cat /etc/resolv.conf

# DNS debugging with dig
kubectl exec <pod-name> -n monitoring -- dig kubernetes.default.svc.cluster.local
```

#### Service Discovery Testing
```bash
# List all services in namespace
kubectl get svc -n monitoring

# Detailed service information
kubectl describe svc <service-name> -n monitoring

# Service endpoints
kubectl get endpoints -n monitoring

# Test service connectivity
kubectl exec <source-pod> -n monitoring -- curl -I http://<service-name>:<port>
```

### Inter-Pod Communication

#### Network Policy Analysis
```bash
# Check network policies (if any)
kubectl get networkpolicy -n monitoring

# Describe network policies
kubectl describe networkpolicy -n monitoring

# Test pod-to-pod connectivity
kubectl exec <pod-a> -n monitoring -- curl -I http://<pod-b-ip>:<port>
```

#### Service Mesh and Load Balancing
```bash
# Check service configuration
kubectl get svc <service-name> -n monitoring -o yaml

# Test load balancing across multiple pods
for i in {1..10}; do kubectl exec <test-pod> -n monitoring -- curl -s http://<service-name>:<port>/health; done

# Check service selector matching
kubectl get pods -n monitoring --show-labels | grep <service-selector>
```

### External Connectivity

#### Ingress Configuration
```bash
# Check ingress resources
kubectl get ingress -n monitoring

# Detailed ingress configuration
kubectl describe ingress <ingress-name> -n monitoring

# Ingress controller logs
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller

# Test ingress routing
curl -H "Host: <hostname>" http://localhost/
```

#### External Service Access
```bash
# Test external API connectivity from pods
kubectl exec <pod-name> -n monitoring -- curl -I http://192.168.0.45:9200
kubectl exec <pod-name> -n monitoring -- curl -I https://api.github.com

# Network route analysis
kubectl exec <pod-name> -n monitoring -- ip route
kubectl exec <pod-name> -n monitoring -- traceroute 8.8.8.8

# DNS server testing
kubectl exec <pod-name> -n monitoring -- nslookup google.com
```

### Port Forwarding and Direct Access

#### Local Development Access
```bash
# Port forward for direct access
kubectl port-forward svc/<service-name> <local-port>:<service-port> -n monitoring

# Port forward specific pod
kubectl port-forward pod/<pod-name> <local-port>:<container-port> -n monitoring

# Background port forwarding
kubectl port-forward svc/<service-name> <local-port>:<service-port> -n monitoring &

# Kill background port forward
pkill -f "kubectl port-forward"
```

#### Network Debugging Tools
```bash
# Install network debugging tools in pod
kubectl exec <pod-name> -n monitoring -- apt update && apt install -y curl dnsutils iputils-ping

# Network interface information
kubectl exec <pod-name> -n monitoring -- ip addr show
kubectl exec <pod-name> -n monitoring -- ip link show

# Network statistics
kubectl exec <pod-name> -n monitoring -- netstat -tlnp
kubectl exec <pod-name> -n monitoring -- ss -tlnp
```

---

## 3. Persistent Volume and Storage Troubleshooting

### PVC Status and Health

#### Volume Claim Information
```bash
# List all PVCs in namespace
kubectl get pvc -n monitoring

# Detailed PVC information
kubectl describe pvc <pvc-name> -n monitoring

# PVC events and issues
kubectl get events -n monitoring --field-selector involvedObject.kind=PersistentVolumeClaim

# Storage class information
kubectl get storageclass
kubectl describe storageclass <storage-class-name>
```

#### Persistent Volume Analysis
```bash
# List all PVs
kubectl get pv

# Detailed PV information
kubectl describe pv <pv-name>

# PV to PVC binding verification
kubectl get pv -o custom-columns=NAME:.metadata.name,CLAIM:.spec.claimRef.name,STATUS:.status.phase

# Check PV access modes and capacity
kubectl get pv -o custom-columns=NAME:.metadata.name,CAPACITY:.spec.capacity.storage,ACCESS:.spec.accessModes
```

### Storage Performance and Usage

#### Disk Usage Analysis
```bash
# Check disk usage inside pods
kubectl exec <pod-name> -n monitoring -- df -h

# Check specific mount point usage
kubectl exec <pod-name> -n monitoring -- du -sh /var/lib/postgresql/data
kubectl exec <pod-name> -n monitoring -- du -sh /opt/sonarqube/data

# Inode usage
kubectl exec <pod-name> -n monitoring -- df -i
```

#### Storage I/O Performance
```bash
# Disk I/O statistics
kubectl exec <pod-name> -n monitoring -- iostat -x 1 5

# File system performance test
kubectl exec <pod-name> -n monitoring -- dd if=/dev/zero of=/tmp/test bs=1M count=100
kubectl exec <pod-name> -n monitoring -- rm /tmp/test

# Check mount options
kubectl exec <pod-name> -n monitoring -- mount | grep <pvc-path>
```

### Common Storage Issues

#### PVC Stuck in Pending
```bash
# Check why PVC is pending
kubectl describe pvc <pvc-name> -n monitoring

# Check storage class availability
kubectl get storageclass

# Check node storage capacity
kubectl describe nodes | grep -A 5 Capacity

# Manual PV creation (if needed)
kubectl get pvc <pvc-name> -n monitoring -o yaml
```

#### Mount Failures
```bash
# Check pod events for mount errors
kubectl describe pod <pod-name> -n monitoring | grep -A 10 Events

# Check volume mount configuration
kubectl get pod <pod-name> -n monitoring -o yaml | grep -A 20 volumeMounts

# Verify volume availability
kubectl exec <pod-name> -n monitoring -- ls -la /mount/path
kubectl exec <pod-name> -n monitoring -- touch /mount/path/test-file
```

#### Data Corruption or Loss
```bash
# Check file system integrity
kubectl exec <pod-name> -n monitoring -- fsck /dev/<device>

# Verify data integrity for databases
kubectl exec postgres-xxx -n monitoring -- pg_dump sonarqube > backup.sql

# Check application-specific data integrity
kubectl exec kafka-0 -n monitoring -- kafka-log-dirs.sh --describe --bootstrap-server localhost:9092
```

---

## 4. Container and Pod Networking Deep Dive

### Network Namespace Analysis

#### Pod Network Configuration
```bash
# Check pod IP and network configuration
kubectl get pods -n monitoring -o wide

# Network namespace information
kubectl exec <pod-name> -n monitoring -- ip addr show
kubectl exec <pod-name> -n monitoring -- ip route show

# Network interface statistics
kubectl exec <pod-name> -n monitoring -- cat /proc/net/dev
```

#### Container Network Troubleshooting
```bash
# Test inter-container communication within pod
kubectl exec <multi-container-pod> -c <container1> -n monitoring -- curl http://localhost:<port>

# Check shared network namespace (same pod containers)
kubectl exec <pod-name> -c <container1> -n monitoring -- netstat -tlnp
kubectl exec <pod-name> -c <container2> -n monitoring -- netstat -tlnp

# Verify shared IP address
kubectl exec <pod-name> -c <container1> -n monitoring -- hostname -I
kubectl exec <pod-name> -c <container2> -n monitoring -- hostname -I
```

### Service and Endpoint Troubleshooting

#### Service Configuration Validation
```bash
# Check service selector matching
kubectl get svc <service-name> -n monitoring -o yaml
kubectl get pods -n monitoring --show-labels | grep <label-selector>

# Endpoint verification
kubectl get endpoints <service-name> -n monitoring
kubectl describe endpoints <service-name> -n monitoring

# Service port mapping verification
kubectl exec <test-pod> -n monitoring -- nc -zv <service-name> <port>
```

#### Load Balancer and Traffic Distribution
```bash
# Test load balancing across replicas
for i in {1..20}; do kubectl exec <test-pod> -n monitoring -- curl -s http://<service-name>:<port>/hostname; done

# Check service proxy mode
kubectl exec <test-pod> -n monitoring -- cat /proc/net/ip_vs

# iptables rules (if using iptables proxy mode)
kubectl exec <test-pod> -n monitoring -- iptables -t nat -L | grep <service-name>
```

### CNI and Network Plugin Issues

#### Container Network Interface Analysis
```bash
# Check CNI configuration
kubectl exec <pod-name> -n monitoring -- cat /etc/cni/net.d/*

# Network plugin logs (varies by implementation)
kubectl logs -n kube-system -l k8s-app=calico-node
kubectl logs -n kube-system -l name=weave-net

# IP allocation and routing
kubectl exec <pod-name> -n monitoring -- ip route get <target-ip>
```

---

## 5. Advanced Troubleshooting Scenarios

### Multi-Container Pod Issues (Aggregator Example)

#### Sidecar Pattern Troubleshooting
```bash
# Check both containers in aggregator pod
kubectl logs aggregator-xxx -c aggregator -n monitoring --tail=20
kubectl logs aggregator-xxx -c filebeat -n monitoring --tail=20

# Shared volume verification
kubectl exec aggregator-xxx -c aggregator -n monitoring -- ls -la /var/log/
kubectl exec aggregator-xxx -c filebeat -n monitoring -- ls -la /var/log/

# Inter-container communication test
kubectl exec aggregator-xxx -c aggregator -n monitoring -- echo "test" >> /var/log/aggregator.log
kubectl exec aggregator-xxx -c filebeat -n monitoring -- tail -1 /var/log/aggregator.log
```

#### Resource Sharing Analysis
```bash
# Check resource allocation per container
kubectl describe pod aggregator-xxx -n monitoring | grep -A 10 "Limits\|Requests"

# Container-specific resource usage
kubectl exec aggregator-xxx -c aggregator -n monitoring -- cat /sys/fs/cgroup/memory.current
kubectl exec aggregator-xxx -c filebeat -n monitoring -- cat /sys/fs/cgroup/memory.current

# Network namespace sharing verification
kubectl exec aggregator-xxx -c aggregator -n monitoring -- ip addr
kubectl exec aggregator-xxx -c filebeat -n monitoring -- ip addr
```

### StatefulSet Networking (Kafka Example)

#### Pod Identity and Discovery
```bash
# Check StatefulSet pod naming and ordering
kubectl get pods -l app=kafka -n monitoring -o wide

# DNS resolution for StatefulSet pods
kubectl exec <test-pod> -n monitoring -- nslookup kafka-0.kafka.monitoring.svc.cluster.local
kubectl exec <test-pod> -n monitoring -- nslookup kafka-1.kafka.monitoring.svc.cluster.local

# Headless service verification
kubectl get svc kafka -n monitoring
kubectl describe svc kafka -n monitoring
```

#### Cluster Formation Testing
```bash
# Test Kafka broker connectivity
kubectl exec kafka-0 -n monitoring -- kafka-broker-api-versions.sh --bootstrap-server kafka-0:9092
kubectl exec kafka-1 -n monitoring -- kafka-broker-api-versions.sh --bootstrap-server kafka-1:9092

# Cross-broker communication
kubectl exec kafka-0 -n monitoring -- kafka-topics.sh --list --bootstrap-server kafka-1:9092,kafka-2:9092
```

---

## 6. Storage Troubleshooting Deep Dive

### PVC Lifecycle Management

#### PVC Creation and Binding Issues
```bash
# Check PVC status and events
kubectl get pvc -n monitoring
kubectl describe pvc <pvc-name> -n monitoring

# Check available storage classes
kubectl get storageclass
kubectl describe storageclass <storage-class>

# Manual PV creation for debugging
kubectl get pvc <pvc-name> -n monitoring -o yaml > pvc-debug.yaml
```

#### Volume Mount Failures
```bash
# Check mount status in pod
kubectl describe pod <pod-name> -n monitoring | grep -A 20 Volumes

# Verify mount permissions
kubectl exec <pod-name> -n monitoring -- ls -la /mount/point
kubectl exec <pod-name> -n monitoring -- touch /mount/point/write-test

# Check mount options and filesystem
kubectl exec <pod-name> -n monitoring -- mount | grep <mount-point>
kubectl exec <pod-name> -n monitoring -- stat -f /mount/point
```

### Data Persistence Verification

#### Database Storage (PostgreSQL Example)
```bash
# Check PostgreSQL data directory
kubectl exec postgres-xxx -n monitoring -- ls -la /var/lib/postgresql/data/

# Database connectivity and data integrity
kubectl exec postgres-xxx -n monitoring -- pg_isready -U sonarqube
kubectl exec postgres-xxx -n monitoring -- psql -U sonarqube -c "\l"

# Check database file permissions
kubectl exec postgres-xxx -n monitoring -- ls -la /var/lib/postgresql/data/base/
```

#### Application Data Storage (SonarQube Example)
```bash
# Check SonarQube data and extensions
kubectl exec sonarqube-xxx -n monitoring -- ls -la /opt/sonarqube/data/
kubectl exec sonarqube-xxx -n monitoring -- ls -la /opt/sonarqube/extensions/

# Verify write permissions
kubectl exec sonarqube-xxx -n monitoring -- touch /opt/sonarqube/data/test-write
kubectl exec sonarqube-xxx -n monitoring -- rm /opt/sonarqube/data/test-write
```

#### Log Storage (Kafka/VictoriaMetrics Example)
```bash
# Check Kafka log segments
kubectl exec kafka-0 -n monitoring -- ls -la /var/lib/kafka/data/
kubectl exec kafka-0 -n monitoring -- du -sh /var/lib/kafka/data/*

# VictoriaMetrics storage
kubectl exec victoria-metrics-xxx -n monitoring -- ls -la /victoria-metrics-data/
kubectl exec victoria-metrics-xxx -n monitoring -- du -sh /victoria-metrics-data/
```

### Storage Performance Analysis

#### I/O Performance Testing
```bash
# Sequential write performance
kubectl exec <pod-name> -n monitoring -- dd if=/dev/zero of=/mount/point/test bs=1M count=100 oflag=direct

# Random I/O performance
kubectl exec <pod-name> -n monitoring -- fio --name=random-write --ioengine=posixaio --rw=randwrite --bs=4k --size=100M --filename=/mount/point/test-file

# Clean up test files
kubectl exec <pod-name> -n monitoring -- rm /mount/point/test*
```

#### Storage Capacity Management
```bash
# Check storage usage across all PVCs
kubectl get pvc -n monitoring -o custom-columns=NAME:.metadata.name,CAPACITY:.spec.resources.requests.storage,STATUS:.status.phase

# Monitor storage growth trends
watch -n 60 'kubectl exec <pod-name> -n monitoring -- df -h /mount/point'

# Storage cleanup and maintenance
kubectl exec <pod-name> -n monitoring -- find /mount/point -type f -mtime +7 -delete
```

---

## 7. Network Security and Isolation

### Network Policy Testing

#### Policy Validation
```bash
# Test network policy enforcement
kubectl exec <source-pod> -n monitoring -- nc -zv <target-pod-ip> <blocked-port>
kubectl exec <source-pod> -n monitoring -- nc -zv <target-pod-ip> <allowed-port>

# Check policy selectors
kubectl get networkpolicy -n monitoring -o yaml

# Verify pod labels match policy
kubectl get pods -n monitoring --show-labels | grep <policy-selector>
```

#### Firewall and Security Groups
```bash
# Check iptables rules (if accessible)
kubectl exec <pod-name> -n monitoring -- iptables -L -n

# Security context verification
kubectl get pod <pod-name> -n monitoring -o yaml | grep -A 10 securityContext

# Check running processes and capabilities
kubectl exec <pod-name> -n monitoring -- ps aux
kubectl exec <pod-name> -n monitoring -- capsh --print
```

---

## 8. Diagnostic Automation Scripts

### Comprehensive Health Check Script
```bash
#!/bin/bash
# k8s_health_check.sh

NAMESPACE="monitoring"

echo "=== Kubernetes Health Check ==="
echo "Timestamp: $(date)"
echo

echo "1. Cluster Status:"
kubectl cluster-info --request-timeout=10s

echo
echo "2. Node Health:"
kubectl get nodes --no-headers | while read node rest; do
    echo "Node: $node"
    kubectl describe node $node | grep -E "(Ready|MemoryPressure|DiskPressure|PIDPressure)"
done

echo
echo "3. Pod Status in $NAMESPACE:"
kubectl get pods -n $NAMESPACE --no-headers | while read pod ready status restarts age; do
    if [ "$status" != "Running" ]; then
        echo "ISSUE: $pod is $status"
        kubectl describe pod $pod -n $NAMESPACE | tail -10
    fi
done

echo
echo "4. PVC Status:"
kubectl get pvc -n $NAMESPACE --no-headers | while read pvc status vol capacity access storageclass age; do
    if [ "$status" != "Bound" ]; then
        echo "ISSUE: PVC $pvc is $status"
    fi
done

echo
echo "5. Service Connectivity:"
for svc in kafka logstash kibana grafana postgres sonarqube; do
    if kubectl get svc $svc -n $NAMESPACE >/dev/null 2>&1; then
        echo "Testing $svc connectivity..."
        kubectl exec deployment/logstash -n $NAMESPACE -- timeout 5 nc -zv $svc $(kubectl get svc $svc -n $NAMESPACE -o jsonpath='{.spec.ports[0].port}') 2>&1
    fi
done
```

### Network Connectivity Test Script
```bash
#!/bin/bash
# network_test.sh

NAMESPACE="monitoring"
TEST_POD="logstash"

echo "=== Network Connectivity Test ==="

# Internal service connectivity
echo "1. Internal Service Tests:"
for service in kafka victoria-metrics postgres sonarqube; do
    if kubectl get svc $service -n $NAMESPACE >/dev/null 2>&1; then
        port=$(kubectl get svc $service -n $NAMESPACE -o jsonpath='{.spec.ports[0].port}')
        echo "Testing $service:$port..."
        kubectl exec deployment/$TEST_POD -n $NAMESPACE -- timeout 5 curl -s -o /dev/null -w "%{http_code} %{time_total}s\n" http://$service:$port/ 2>/dev/null || echo "Connection failed"
    fi
done

# External connectivity
echo
echo "2. External Connectivity:"
echo "Testing Elasticsearch VM..."
kubectl exec deployment/$TEST_POD -n $NAMESPACE -- timeout 5 curl -s -o /dev/null -w "%{http_code} %{time_total}s\n" http://192.168.0.45:9200 2>/dev/null || echo "ES VM unreachable"

echo "Testing internet connectivity..."
kubectl exec deployment/$TEST_POD -n $NAMESPACE -- timeout 5 curl -s -o /dev/null -w "%{http_code} %{time_total}s\n" http://google.com 2>/dev/null || echo "Internet unreachable"

# DNS resolution
echo
echo "3. DNS Resolution:"
kubectl exec deployment/$TEST_POD -n $NAMESPACE -- nslookup kubernetes.default
kubectl exec deployment/$TEST_POD -n $NAMESPACE -- nslookup kafka.monitoring.svc.cluster.local
```

### Storage Health Check Script
```bash
#!/bin/bash
# storage_health.sh

NAMESPACE="monitoring"

echo "=== Storage Health Check ==="

# PVC status check
echo "1. PVC Status:"
kubectl get pvc -n $NAMESPACE -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,CAPACITY:.spec.resources.requests.storage,USED:.status.capacity.storage

# Disk usage check for each pod with PVC
echo
echo "2. Disk Usage by Pod:"
for pod in $(kubectl get pods -n $NAMESPACE -o jsonpath='{.items[*].metadata.name}'); do
    if kubectl describe pod $pod -n $NAMESPACE | grep -q "ClaimName"; then
        echo "Pod: $pod"
        kubectl exec $pod -n $NAMESPACE -- df -h 2>/dev/null | grep -E "(Filesystem|/var/lib|/opt)" || echo "  Unable to check disk usage"
        echo
    fi
done

# Storage performance check
echo "3. Storage Performance Test:"
kubectl exec postgres-xxx -n $NAMESPACE -- timeout 10 dd if=/dev/zero of=/var/lib/postgresql/data/test bs=1M count=10 2>&1 | grep -E "(MB/s|copied)"
kubectl exec postgres-xxx -n $NAMESPACE -- rm -f /var/lib/postgresql/data/test

echo
echo "4. Storage Events:"
kubectl get events -n $NAMESPACE --field-selector involvedObject.kind=PersistentVolumeClaim --sort-by='.lastTimestamp' | tail -10
```

---

## 9. Emergency Response Procedures

### Rapid Diagnostics

#### Quick Problem Identification
```bash
# One-liner cluster overview
kubectl get nodes,pods -n monitoring --no-headers | grep -v Running

# Critical pod restart identification
kubectl get pods -n monitoring --no-headers | awk '$4 > 5 {print $1 " has " $4 " restarts"}'

# Recent error events
kubectl get events -n monitoring --field-selector type=Warning --sort-by='.lastTimestamp' | tail -5

# Resource pressure identification
kubectl top pods -n monitoring --sort-by=memory | tail -5
```

#### Emergency Recovery Commands
```bash
# Force pod restart
kubectl delete pod <problematic-pod> -n monitoring

# Rolling restart for deployments
kubectl rollout restart deployment/<deployment-name> -n monitoring

# Scale down and up
kubectl scale deployment <deployment-name> --replicas=0 -n monitoring
kubectl scale deployment <deployment-name> --replicas=1 -n monitoring

# Force PVC remount
kubectl delete pod <pod-name> -n monitoring
# Pod will be recreated by deployment controller
```

### Data Recovery Procedures

#### Database Recovery
```bash
# PostgreSQL recovery
kubectl exec postgres-xxx -n monitoring -- pg_dump sonarqube > emergency-backup.sql
kubectl cp postgres-xxx:/tmp/backup.sql ./postgres-recovery.sql -n monitoring

# Check database corruption
kubectl exec postgres-xxx -n monitoring -- pg_checksums --check -D /var/lib/postgresql/data
```

#### Index Recovery
```bash
# Elasticsearch index recovery
curl -X POST "http://192.168.0.45:9200/_cluster/reroute?retry_failed=true&pretty"

# Force allocation of unassigned shards
curl -X POST "http://192.168.0.45:9200/_cluster/reroute?pretty" \
  -H "Content-Type: application/json" \
  -d '{"commands":[{"allocate_replica":{"index":"<index-name>","shard":0,"node":"<node-name>"}}]}'
```

---

## 10. Monitoring and Alerting Integration

### Automated Monitoring Setup

#### Health Check Automation
```bash
# Cron job for regular health checks
cat > k8s-monitor.sh << 'EOF'
#!/bin/bash
LOG_FILE="/tmp/k8s-health-$(date +%Y%m%d).log"

{
    echo "=== Health Check: $(date) ==="
    kubectl get pods -n monitoring | grep -v Running
    curl -s "http://192.168.0.45:9200/_cluster/health" | jq '.status'
    echo "=== End Check ==="
} >> $LOG_FILE

# Alert if issues found
if kubectl get pods -n monitoring | grep -v Running | grep -v RESTARTS; then
    echo "K8s pods have issues" | mail -s "K8s Alert" admin@company.com
fi
EOF

# Add to crontab
# */5 * * * * /path/to/k8s-monitor.sh
```

#### Performance Baselines
```bash
# Establish performance baselines
echo "=== Performance Baseline ==="
kubectl top pods -n monitoring
curl -s "http://192.168.0.45:9200/_nodes/stats/jvm" | jq '.nodes[] | .jvm.mem.heap_used_percent'
curl -s "http://192.168.0.45:9200/_cat/indices/aggregator-logs-*?h=docs.count" | awk '{sum+=$1} END {print "Total docs: " sum}'
```

### Integration with External Monitoring

#### Metrics Export for Prometheus
```bash
# Kubernetes metrics
kubectl get --raw /metrics

# Individual pod metrics
kubectl exec <pod-name> -n monitoring -- curl http://localhost:<metrics-port>/metrics

# Custom metrics from applications
kubectl exec aggregator-xxx -c aggregator -n monitoring -- curl http://localhost:8080/metrics
```

---

## 11. Best Practices and Operational Guidelines

### Regular Maintenance Tasks

#### Daily Operations
```bash
# Daily health check routine
kubectl get pods -n monitoring
curl -I http://192.168.0.45:9200
kubectl get pvc -n monitoring

# Log rotation and cleanup
kubectl logs deployment/logstash -n monitoring --tail=1000 > daily-logstash-$(date +%Y%m%d).log

# Resource usage trending
kubectl top pods -n monitoring > daily-resources-$(date +%Y%m%d).txt
```

#### Weekly Maintenance
```bash
# Check for pod restart trends
kubectl get pods -n monitoring -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.containerStatuses[0].restartCount}{"\n"}{end}'

# Storage usage trends
for pvc in $(kubectl get pvc -n monitoring -o jsonpath='{.items[*].metadata.name}'); do
    echo "PVC: $pvc"
    kubectl get pvc $pvc -n monitoring -o jsonpath='{.status.capacity.storage}{"\n"}'
done

# Network performance check
kubectl exec deployment/logstash -n monitoring -- time curl -s http://192.168.0.45:9200/_cluster/health
```

### Documentation and Change Management

#### Configuration Backup
```bash
# Backup all configurations
kubectl get all,configmap,secret,pvc,ingress -n monitoring -o yaml > monitoring-backup-$(date +%Y%m%d).yaml

# Backup specific components
kubectl get configmap logstash-pipeline -n monitoring -o yaml > logstash-config-backup.yaml
kubectl get deployment aggregator -n monitoring -o yaml > aggregator-deployment-backup.yaml
```

#### Change Tracking
```bash
# Track configuration changes
kubectl annotate deployment logstash -n monitoring change-log="Updated configuration $(date)"

# Check recent changes
kubectl get events -n monitoring --sort-by='.lastTimestamp' | grep -E "(deployment|configmap)"

# Configuration validation
kubectl apply --dry-run=client -f <config-file.yaml>
```

This comprehensive guide provides systematic approaches to Kubernetes health monitoring, network troubleshooting, and storage management specifically tailored for your GoMon infrastructure.