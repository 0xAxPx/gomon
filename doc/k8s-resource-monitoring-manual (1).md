# Kubernetes Resource Utilization Detection Manual
## Essential Guide for SRE Day-to-Day Operations & Interview Preparation

**Project**: GoMon Infrastructure  
**Target**: Production-ready resource monitoring skills  
**Environment**: Any Kubernetes cluster (Docker Desktop, EKS, GKE, etc.)  
**Use Case**: Daily operations, troubleshooting, SRE interviews  
**Last Updated**: September 14, 2025

---

## 🎯 **Core Resource Monitoring Commands**

### **1. Cluster-Level Resource Overview**

```bash
# Quick cluster health check
kubectl top nodes

# Example output:
NAME             CPU(cores)   CPU%   MEMORY(bytes)   MEMORY%   
docker-desktop   1259m        15%    3456Mi          43%
```

**What this tells you:**
- **CPU(cores)**: Actual CPU usage across all pods on this node
- **CPU%**: Percentage of node's total CPU capacity
- **MEMORY(bytes)**: Actual memory usage across all pods
- **MEMORY%**: Percentage of node's total memory capacity

### **2. Pod-Level Resource Analysis**

```bash
# All pods resource usage
kubectl top pods -n monitoring

# Specific pod with containers breakdown
kubectl top pods -n monitoring --containers

# Sort by memory usage (highest first)
kubectl top pods -n monitoring --sort-by=memory

# Sort by CPU usage
kubectl top pods -n monitoring --sort-by=cpu
```

**Example output analysis:**
```bash
NAME                          CPU(cores)   MEMORY(bytes)   
aggregator-74df69d946-78jzs   15m          245Mi
kafka-0                       45m          512Mi
logstash-58474d4db-rzmmk      67m          1024Mi
```

---

## 🔍 **Deep Resource Investigation**

### **3. Resource Requests vs Limits vs Actual Usage**

```bash
# See configured requests and limits
kubectl describe pod <pod-name> -n monitoring

# Example interpretation:
Containers:
  aggregator:
    Limits:      # Maximum allowed
      cpu:     200m
      memory:  512Mi
    Requests:    # Guaranteed minimum
      cpu:     100m
      memory:  256Mi
```

**Critical Understanding:**
- **Requests**: Kubernetes guarantees this amount
- **Limits**: Pod gets killed if it exceeds this
- **Actual Usage**: What the pod is currently using (from `kubectl top`)

### **4. Resource Pressure Detection**

```bash
# Check for resource pressure on nodes
kubectl describe nodes | grep -A 5 "Conditions:"

# Look for:
# MemoryPressure: False/True
# DiskPressure: False/True  
# PIDPressure: False/True
```

**Example warning signs:**
```bash
Conditions:
  MemoryPressure    True     # ⚠️ Node running low on memory
  DiskPressure      False    # ✅ Disk space OK
  PIDPressure       False    # ✅ Process limit OK
```

---

## 🚨 **Resource Troubleshooting Scenarios**

### **Scenario 1: Pod Keeps Getting Killed (OOMKilled)**

```bash
# Check recent pod events
kubectl get events -n monitoring --field-selector involvedObject.name=<pod-name>

# Look for events like:
# Killing container ... (reason: OOMKilled)

# Check current memory usage vs limits
kubectl top pod <pod-name> -n monitoring --containers
kubectl describe pod <pod-name> -n monitoring | grep -A 3 Limits
```

**Analysis Process:**
1. **Compare**: Actual memory usage vs memory limit
2. **Historical**: Check if memory usage is growing over time
3. **Fix**: Increase memory limit or optimize application

### **Scenario 2: CPU Throttling Issues**

```bash
# Check CPU usage patterns
kubectl top pods -n monitoring --sort-by=cpu

# Check for CPU throttling in container stats
kubectl exec <pod-name> -n monitoring -- cat /sys/fs/cgroup/cpu.stat

# Look for:
# nr_throttled: 1234    # Number of times throttled
# throttled_time: 5000  # Total time throttled (nanoseconds)
```

### **Scenario 3: Cluster Resource Exhaustion**

```bash
# Check node capacity vs allocatable
kubectl describe nodes

# Look for:
Capacity:
  cpu:     4
  memory:  8Gi
Allocatable:  # What's actually available for pods
  cpu:     3800m
  memory:  7Gi

# Check total requests across all pods
kubectl describe nodes | grep -A 4 "Allocated resources"

# Example output:
Allocated resources:
  cpu:     2100m (55% of capacity)
  memory:  4Gi (57% of capacity)
```

---

## 📊 **Advanced Resource Monitoring**

### **5. Resource Usage Over Time**

```bash
# Monitor resource usage continuously
watch -n 5 'kubectl top pods -n monitoring'

# Check resource usage for specific time period
kubectl top pods -n monitoring --since-time=2025-09-14T10:00:00Z

# Save resource snapshot for analysis
kubectl top pods -n monitoring > resource-snapshot-$(date +%Y%m%d-%H%M).txt
```

### **6. Container-Level Resource Deep Dive**

```bash
# For Docker Desktop / containerd
kubectl exec <pod-name> -n monitoring -- cat /proc/meminfo
kubectl exec <pod-name> -n monitoring -- cat /proc/loadavg

# Memory breakdown
kubectl exec <pod-name> -n monitoring -- cat /sys/fs/cgroup/memory.current
kubectl exec <pod-name> -n monitoring -- cat /sys/fs/cgroup/memory.max

# CPU usage details
kubectl exec <pod-name> -n monitoring -- cat /sys/fs/cgroup/cpu.stat
```

---

## 🎭 **SRE Interview Scenarios**

### **Interview Question 1: "A pod keeps restarting. How do you investigate?"**

**Your Answer Process:**
```bash
# 1. Check pod status and restart count
kubectl get pods -n monitoring

# 2. Check recent events
kubectl describe pod <pod-name> -n monitoring

# 3. Check logs from previous restart
kubectl logs <pod-name> -n monitoring --previous

# 4. Check resource usage vs limits
kubectl top pod <pod-name> -n monitoring --containers
kubectl describe pod <pod-name> -n monitoring | grep -A 6 Limits

# 5. Check for OOMKilled events
kubectl get events -n monitoring | grep OOMKilling
```

### **Interview Question 2: "Cluster is slow. How do you identify the bottleneck?"**

**Your Investigation Process:**
```bash
# 1. Cluster overview
kubectl top nodes

# 2. Identify resource-heavy pods
kubectl top pods --all-namespaces --sort-by=cpu
kubectl top pods --all-namespaces --sort-by=memory

# 3. Check for pending pods (resource constraints)
kubectl get pods --all-namespaces | grep Pending

# 4. Node capacity analysis
kubectl describe nodes | grep -A 4 "Allocated resources"

# 5. Check for resource pressure
kubectl describe nodes | grep -A 5 Conditions
```

### **Interview Question 3: "How do you right-size pod resources?"**

**Your Methodology:**
```bash
# 1. Baseline current usage
kubectl top pods -n monitoring --containers

# 2. Historical analysis (if monitoring available)
# Check Grafana/Prometheus for usage patterns over 1 week

# 3. Compare with requests/limits
kubectl describe pod <pod-name> -n monitoring | grep -A 6 -B 6 Limits

# 4. Calculate optimal values:
# - Requests: 90th percentile of actual usage
# - Limits: 2x requests (with safety margin)
```

---

## 🛠️ **Practical Resource Management**

### **7. Resource Optimization Examples**

**Before optimization:**
```yaml
resources:
  requests:
    cpu: 100m     # Pod actually uses 45m average
    memory: 256Mi # Pod actually uses 120Mi average
  limits:
    cpu: 200m     # Never hit this limit
    memory: 512Mi # Sometimes hits 245Mi peak
```

**After optimization:**
```yaml
resources:
  requests:
    cpu: 50m      # Based on 90th percentile actual usage
    memory: 150Mi # Based on average + buffer
  limits:
    cpu: 150m     # 3x requests for CPU bursts
    memory: 300Mi # 2x requests with safety margin
```

### **8. Emergency Resource Management**

```bash
# Quickly identify resource hogs
kubectl top pods --all-namespaces --sort-by=memory | head -10

# Emergency pod restart (if OOMKilled)
kubectl delete pod <pod-name> -n monitoring

# Temporary resource increase (for emergencies)
kubectl patch deployment <deployment-name> -n monitoring -p='{"spec":{"template":{"spec":{"containers":[{"name":"<container-name>","resources":{"limits":{"memory":"1Gi"}}}]}}}}'

# Scale down resource-heavy deployment temporarily
kubectl scale deployment <deployment-name> --replicas=0 -n monitoring
```

---

## 📋 **Daily SRE Resource Monitoring Checklist**

### **Morning Health Check (5 minutes)**
```bash
# 1. Overall cluster health
kubectl top nodes

# 2. Identify any resource pressure
kubectl get pods --all-namespaces | grep -E "(OOMKilled|Pending|Error)"

# 3. Check top resource consumers
kubectl top pods --all-namespaces --sort-by=memory | head -5
kubectl top pods --all-namespaces --sort-by=cpu | head -5

# 4. Verify critical services
kubectl get pods -n monitoring
```

### **Weekly Resource Review (15 minutes)**
```bash
# 1. Resource trend analysis
kubectl top pods -n monitoring > weekly-resources-$(date +%Y%m%d).txt

# 2. Compare with previous week
diff weekly-resources-$(date -d '7 days ago' +%Y%m%d).txt weekly-resources-$(date +%Y%m%d).txt

# 3. Identify pods that need resource tuning
# (Compare actual usage vs configured requests/limits)

# 4. Check for cluster capacity planning
kubectl describe nodes | grep "Allocated resources" -A 4
```

---

## 🎯 **Key Metrics to Monitor (SRE Interview Ready)**

### **Node-Level Metrics**
- **CPU Utilization**: <80% average, <90% peak
- **Memory Utilization**: <80% average, <85% peak  
- **Disk Utilization**: <85% for system disk
- **Network I/O**: Monitor for saturation

### **Pod-Level Metrics**
- **Memory Usage vs Limit**: Should be <80% of limit
- **CPU Usage vs Request**: Should average around request value
- **Restart Count**: Monitor for increasing restarts
- **Ready State**: Pods should be Ready=True

### **Resource Efficiency Metrics**
- **Request Utilization**: Actual usage / Requested resources
- **Waste Ratio**: (Requested - Actual) / Requested
- **Over-commitment Ratio**: Total requests / Node capacity

---

## 🔧 **GoMon Project Specific Commands**

### **Quick GoMon Health Check**
```bash
# Check all monitoring pods
kubectl get pods -n monitoring

# Resource usage for GoMon components
kubectl top pods -n monitoring --sort-by=memory

# Specific checks for key services
kubectl top pod -n monitoring --selector=app=aggregator --containers
kubectl top pod -n monitoring --selector=app=kafka --containers
kubectl top pod -n monitoring --selector=app=logstash --containers
```

### **GoMon Resource Analysis Example**
Based on actual GoMon cluster metrics (September 14, 2025):

```bash
NAMESPACE   NAME                              CPU(cores)   MEMORY(bytes)   
monitoring  kafka-2                           21m          452Mi           
monitoring  kafka-1                           20m          414Mi           
monitoring  kafka-0                           15m          419Mi           
monitoring  sonarqube-78f7d89fd-9b2s5         13m          2456Mi          
monitoring  kibana-7d7b7869f-2lm6n            13m          749Mi           
monitoring  logstash-5d94d8f59c-vgp57         7m           1513Mi          
monitoring  grafana-5bc997fc98-gfs46          5m           153Mi           
monitoring  postgres-db7b7d7df-5zkwq          3m           176Mi           
monitoring  victoria-metrics-5657c99846-hq77w 2m           81Mi            
monitoring  aggregator-758dfb9544-8stdr       2m           65Mi            
monitoring  elasticsearch-lb-576f749684-wc48q 2m           14Mi            
monitoring  jaeger-57c59fb84-rjwwq            1m           60Mi            
monitoring  agent-67bf65c565-v7w6f            1m           11Mi            
```

**Resource Analysis:**
- **Highest CPU**: Kafka cluster (21m, 20m, 15m per pod)
- **Highest Memory**: SonarQube (2456Mi), Logstash (1513Mi)
- **Most Efficient**: Agent (1m CPU, 11Mi memory)
- **Total Monitoring Namespace**: ~113m CPU, ~6.5Gi memory

### **GoMon Resource Optimization Analysis**
```bash
# Identify over-provisioned resources
kubectl describe pod aggregator-* -n monitoring | grep -A 6 -B 6 Limits
kubectl describe pod kafka-* -n monitoring | grep -A 6 -B 6 Limits
kubectl describe pod logstash-* -n monitoring | grep -A 6 -B 6 Limits

# Check for resource pressure in GoMon namespace
kubectl describe nodes | grep -A 10 "monitoring"

# Monitor resource trends for capacity planning
kubectl top pods -n monitoring --sort-by=memory > gomon-resources-$(date +%Y%m%d).txt
```

---

## 🚀 **Advanced Monitoring Commands**

### **Resource Alerts & Thresholds**
```bash
# Check pods exceeding 80% memory limit
kubectl top pods -n monitoring --no-headers | awk '$3 ~ /Mi/ && $3+0 > 400 {print $1 " exceeds memory threshold: " $3}'

# Monitor CPU throttling across pods
for pod in $(kubectl get pods -n monitoring -o name); do
  echo "=== $pod ==="
  kubectl exec $pod -n monitoring -- cat /sys/fs/cgroup/cpu.stat 2>/dev/null | grep throttled || echo "Cannot access cgroup stats"
done
```

### **Historical Resource Analysis**
```bash
# Create resource snapshot script
cat > resource-monitor.sh << 'EOF'
#!/bin/bash
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
echo "=== Resource Snapshot: $TIMESTAMP ===" >> resource-history.log
kubectl top nodes >> resource-history.log
kubectl top pods -n monitoring >> resource-history.log
echo "" >> resource-history.log
EOF

chmod +x resource-monitor.sh

# Run every 5 minutes via cron
# */5 * * * * /path/to/resource-monitor.sh
```

---

## 📚 **References and Further Reading**

### **Official Documentation**
- [Kubernetes Resource Management](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)
- [kubectl top command reference](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#top)

### **Best Practices**
- **Resource Requests**: Set to 90th percentile of actual usage
- **Resource Limits**: Set to 2-3x requests with safety margin
- **Monitoring Frequency**: Check daily, analyze weekly
- **Alerting Thresholds**: >80% for warnings, >90% for critical

### **Common Issues**
- **OOMKilled**: Increase memory limits or optimize application
- **CPU Throttling**: Increase CPU limits or optimize processing
- **Pending Pods**: Insufficient cluster resources or resource quotas
- **Resource Waste**: Requests much higher than actual usage

---

## 🎯 **Summary for SRE Interviews**

**Key Talking Points:**
1. **Proactive Monitoring**: Regular resource health checks prevent issues
2. **Data-Driven Optimization**: Base resource tuning on actual usage metrics
3. **Systematic Troubleshooting**: Follow consistent investigation processes
4. **Capacity Planning**: Monitor trends to predict future resource needs
5. **Emergency Response**: Know how to quickly identify and resolve resource issues

**Demonstrate Experience:**
- Show familiarity with `kubectl top` commands
- Explain difference between requests, limits, and actual usage
- Describe systematic approach to troubleshooting resource issues
- Discuss resource optimization strategies based on real metrics