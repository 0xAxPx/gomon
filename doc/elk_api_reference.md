# ELK Stack API Reference Guide
## Essential API Calls for Health Monitoring and Management

**Project**: GoMon Infrastructure  
**Elasticsearch**: 192.168.0.45:9200  
**Environment**: Development/Production  

---

## 1. ELK Health Monitoring

### Elasticsearch Cluster Health

#### Basic Cluster Status
```bash
# Overall cluster health
curl -X GET "http://192.168.0.45:9200/_cluster/health?pretty"

# Detailed cluster health with explanations
curl -X GET "http://192.168.0.45:9200/_cluster/health?pretty&level=indices"

# Wait for specific health status
curl -X GET "http://192.168.0.45:9200/_cluster/health/aggregator-logs?wait_for_status=yellow&timeout=50s&pretty"
```

#### Node Information
```bash
# Node details and resource usage
curl -X GET "http://192.168.0.45:9200/_nodes?pretty"

# Node statistics (CPU, memory, disk)
curl -X GET "http://192.168.0.45:9200/_nodes/stats?pretty"

# Specific node health
curl -X GET "http://192.168.0.45:9200/_nodes/stats/jvm,os,process?pretty"
```

#### Task Management
```bash
# Currently running tasks
curl -X GET "http://192.168.0.45:9200/_tasks?pretty"

# Pending cluster tasks
curl -X GET "http://192.168.0.45:9200/_cluster/pending_tasks?pretty"

# Hot threads (for performance debugging)
curl -X GET "http://192.168.0.45:9200/_nodes/hot_threads"
```

### Logstash Health (Kubernetes)

#### Pod Status and Logs
```bash
# Check Logstash pod health
kubectl get pods -l app=logstash -n monitoring

# Recent Logstash logs
kubectl logs deployment/logstash -n monitoring --tail=50

# Logstash connection status
kubectl logs deployment/logstash -n monitoring | grep -i "elasticsearch\|connection\|error"

# Test connectivity from Logstash to ES
kubectl exec deployment/logstash -n monitoring -- curl -I http://192.168.0.45:9200
```

#### Configuration Verification
```bash
# Check current Logstash configuration
kubectl exec deployment/logstash -n monitoring -- cat /usr/share/logstash/pipeline/logstash.conf

# Verify ConfigMap
kubectl get configmap logstash-pipeline -n monitoring -o yaml
```

### Kibana Health (Kubernetes)

#### Service Status
```bash
# Check Kibana pod status
kubectl get pods -l app=kibana -n monitoring

# Kibana startup logs
kubectl logs deployment/kibana -n monitoring --tail=30

# Test Kibana connectivity to ES
kubectl exec deployment/kibana -n monitoring -- curl -I http://192.168.0.45:9200
```

#### Access Verification
```bash
# Test Kibana web interface
curl -I http://kibana.local

# Port-forward for direct access
kubectl port-forward svc/kibana 5601:5601 -n monitoring
```

---

## 2. Index Information and Management

### Index Listing and Statistics

#### Basic Index Information
```bash
# List all indices
curl -X GET "http://192.168.0.45:9200/_cat/indices?v"

# Aggregator-specific indices
curl -X GET "http://192.168.0.45:9200/_cat/indices/aggregator-logs-*?v&s=index"

# Index health and document counts
curl -X GET "http://192.168.0.45:9200/_cat/indices/aggregator-logs-*?v&h=index,health,status,docs.count,store.size&s=index"
```

#### Detailed Index Information
```bash
# Specific index settings
curl -X GET "http://192.168.0.45:9200/aggregator-logs-000002/_settings?pretty"

# Index mapping information
curl -X GET "http://192.168.0.45:9200/aggregator-logs-000002/_mapping?pretty"

# Index statistics
curl -X GET "http://192.168.0.45:9200/aggregator-logs-000002/_stats?pretty"
```

#### Alias Management
```bash
# List all aliases
curl -X GET "http://192.168.0.45:9200/_alias?pretty"

# Specific alias information
curl -X GET "http://192.168.0.45:9200/_alias/aggregator-logs?pretty"

# Indices behind an alias
curl -X GET "http://192.168.0.45:9200/aggregator-logs/_alias?pretty"
```

### Index Content Analysis

#### Document Counting
```bash
# Total documents in alias
curl -X GET "http://192.168.0.45:9200/aggregator-logs/_count?pretty"

# Count with query filter
curl -X GET "http://192.168.0.45:9200/aggregator-logs/_count?pretty" \
  -H "Content-Type: application/json" \
  -d '{"query":{"exists":{"field":"metric_name"}}}'

# Count by specific index
curl -X GET "http://192.168.0.45:9200/aggregator-logs-000002/_count?pretty"
```

#### Data Sampling
```bash
# Latest documents
curl -X GET "http://192.168.0.45:9200/aggregator-logs/_search?pretty&size=5&sort=@timestamp:desc"

# Search for specific metric types
curl -X GET "http://192.168.0.45:9200/aggregator-logs/_search?pretty" \
  -H "Content-Type: application/json" \
  -d '{"query":{"term":{"metric_name.keyword":"cpu_usage_percent"}},"size":5}'

# Aggregations by metric type
curl -X GET "http://192.168.0.45:9200/aggregator-logs/_search?pretty" \
  -H "Content-Type: application/json" \
  -d '{"aggs":{"metric_types":{"terms":{"field":"metric_name.keyword","size":20}}},"size":0}'
```

### Index Performance

#### Shard Information
```bash
# Shard allocation and status
curl -X GET "http://192.168.0.45:9200/_cat/shards/aggregator-logs-*?v"

# Unassigned shards (troubleshooting)
curl -X GET "http://192.168.0.45:9200/_cat/shards?h=index,shard,prirep,state,unassigned.reason&v"

# Shard allocation explanation
curl -X GET "http://192.168.0.45:9200/_cluster/allocation/explain?pretty"
```

#### Recovery Status
```bash
# Index recovery information
curl -X GET "http://192.168.0.45:9200/_cat/recovery/aggregator-logs-*?v"

# Active recovery operations
curl -X GET "http://192.168.0.45:9200/_recovery?pretty&active_only=true"
```

---

## 3. Index Lifecycle Management (ILM)

### ILM Policy Management

#### Policy Information
```bash
# List all ILM policies
curl -X GET "http://192.168.0.45:9200/_ilm/policy?pretty"

# Specific policy details
curl -X GET "http://192.168.0.45:9200/_ilm/policy/aggregator-rollover-policy?pretty"

# ILM service status
curl -X GET "http://192.168.0.45:9200/_ilm/status?pretty"
```

#### Policy Operations
```bash
# Start ILM service
curl -X POST "http://192.168.0.45:9200/_ilm/start?pretty"

# Stop ILM service
curl -X POST "http://192.168.0.45:9200/_ilm/stop?pretty"

# Execute ILM actions immediately
curl -X POST "http://192.168.0.45:9200/_ilm/move/aggregator-logs-000002?pretty" \
  -H "Content-Type: application/json" \
  -d '{"current_step":{"phase":"hot","action":"rollover","name":"attempt-rollover"},"next_step":{"phase":"warm","action":"forcemerge","name":"forcemerge"}}'
```

### Index Lifecycle Status

#### Per-Index ILM Status
```bash
# ILM explain for specific index
curl -X GET "http://192.168.0.45:9200/_ilm/explain/aggregator-logs-000002?pretty"

# All aggregator indices ILM status
curl -X GET "http://192.168.0.45:9200/_ilm/explain/aggregator-logs-*?pretty"

# Filter by lifecycle phase
curl -X GET "http://192.168.0.45:9200/_ilm/explain/aggregator-logs-*?pretty" | jq '.indices[] | select(.phase == "hot")'
```

#### ILM History and Actions
```bash
# ILM execution history
curl -X GET "http://192.168.0.45:9200/aggregator-logs-*/_ilm/explain?only_errors&only_managed&pretty"

# Lifecycle execution details
curl -X GET "http://192.168.0.45:9200/_ilm/explain?pretty" | grep -A 20 "aggregator-logs"
```

### Rollover Management

#### Manual Rollover Operations
```bash
# Force rollover on alias
curl -X POST "http://192.168.0.45:9200/aggregator-logs/_rollover?pretty"

# Rollover with conditions check
curl -X POST "http://192.168.0.45:9200/aggregator-logs/_rollover?pretty" \
  -H "Content-Type: application/json" \
  -d '{"conditions":{"max_age":"1d","max_primary_shard_size":"2mb"}}'

# Rollover dry run (test without executing)
curl -X POST "http://192.168.0.45:9200/aggregator-logs/_rollover?dry_run&pretty"
```

#### Rollover History
```bash
# Check which indices were created by rollover
curl -X GET "http://192.168.0.45:9200/aggregator-logs-*/_settings?pretty" | grep -A 5 rollover_alias

# Rollover statistics
curl -X GET "http://192.168.0.45:9200/_cat/indices/aggregator-logs-*?v&h=index,creation.date.string,docs.count,store.size&s=creation.date"
```

---

## 4. Troubleshooting and Diagnostics

### Performance Analysis

#### Query Performance
```bash
# Slow query log
curl -X GET "http://192.168.0.45:9200/_nodes/stats/indices/search?pretty"

# Cache statistics
curl -X GET "http://192.168.0.45:9200/_nodes/stats/indices/query_cache,request_cache?pretty"

# Thread pool statistics
curl -X GET "http://192.168.0.45:9200/_nodes/stats/thread_pool?pretty"
```

#### Resource Usage
```bash
# JVM heap usage
curl -X GET "http://192.168.0.45:9200/_nodes/stats/jvm?pretty"

# File system usage
curl -X GET "http://192.168.0.45:9200/_nodes/stats/fs?pretty"

# OS statistics
curl -X GET "http://192.168.0.45:9200/_nodes/stats/os?pretty"
```

### Connection Diagnostics

#### Network Testing from Kubernetes
```bash
# Test connectivity from various pods
kubectl exec deployment/logstash -n monitoring -- curl -m 5 -s -o /dev/null -w "%{http_code} %{time_total}s\n" http://192.168.0.45:9200

kubectl exec deployment/kibana -n monitoring -- curl -m 5 -s -o /dev/null -w "%{http_code} %{time_total}s\n" http://192.168.0.45:9200

# Continuous monitoring
kubectl exec deployment/logstash -n monitoring -- sh -c 'while true; do curl -s -o /dev/null -w "$(date): %{http_code} %{time_total}s\n" http://192.168.0.45:9200; sleep 10; done'
```

#### Elasticsearch Connection Monitoring
```bash
# Monitor active connections on ES
curl -X GET "http://192.168.0.45:9200/_nodes/stats/http?pretty"

# Transport statistics
curl -X GET "http://192.168.0.45:9200/_nodes/stats/transport?pretty"
```

---

## 5. Index Templates and Mapping

### Template Management
```bash
# List all index templates
curl -X GET "http://192.168.0.45:9200/_index_template?pretty"

# Specific template details
curl -X GET "http://192.168.0.45:9200/_index_template/aggregator-logs-template?pretty"

# Template simulation (test what mapping would be applied)
curl -X POST "http://192.168.0.45:9200/_index_template/_simulate_index/aggregator-logs-test?pretty"
```

### Field Mapping Analysis
```bash
# Get field mapping for current index
curl -X GET "http://192.168.0.45:9200/aggregator-logs/_mapping?pretty"

# Field capabilities across all indices
curl -X GET "http://192.168.0.45:9200/aggregator-logs-*/_field_caps?fields=metric_name,metric_value,@timestamp&pretty"

# Mapping conflicts detection
curl -X GET "http://192.168.0.45:9200/aggregator-logs-*/_mapping?pretty" | grep -A 5 -B 5 conflict
```

---

## 6. Performance and Maintenance

### Cache and Performance
```bash
# Clear caches
curl -X POST "http://192.168.0.45:9200/_cache/clear?pretty"

# Clear specific index cache
curl -X POST "http://192.168.0.45:9200/aggregator-logs/_cache/clear?pretty"

# Refresh indices
curl -X POST "http://192.168.0.45:9200/aggregator-logs/_refresh?pretty"
```

### Segment Management
```bash
# Segment information
curl -X GET "http://192.168.0.45:9200/_cat/segments/aggregator-logs-*?v"

# Force merge (optimize segments)
curl -X POST "http://192.168.0.45:9200/aggregator-logs-000002/_forcemerge?max_num_segments=1&pretty"

# Monitor merge operations
curl -X GET "http://192.168.0.45:9200/_cat/nodes?h=name,merges.current,merges.total&v"
```

---

## 7. Quick Reference Commands

### Daily Health Check
```bash
#!/bin/bash
# Save as elk_health_check.sh

echo "=== ELK Stack Health Check ==="
echo

echo "1. Cluster Health:"
curl -s "http://192.168.0.45:9200/_cluster/health" | jq '.status, .number_of_nodes, .active_shards'

echo
echo "2. Index Count:"
curl -s "http://192.168.0.45:9200/_cat/indices/aggregator-logs-*?h=index,docs.count,store.size" | sort

echo
echo "3. ILM Status:"
curl -s "http://192.168.0.45:9200/_ilm/status" | jq '.operation_mode'

echo
echo "4. Latest Documents:"
curl -s "http://192.168.0.45:9200/aggregator-logs/_count" | jq '.count'

echo
echo "5. Kubernetes Pods:"
kubectl get pods -n monitoring | grep -E "(logstash|kibana)"
```

### Emergency Diagnostics
```bash
#!/bin/bash
# Save as elk_emergency_debug.sh

echo "=== Emergency ELK Diagnostics ==="

echo "1. ES Connection Test:"
kubectl exec deployment/logstash -n monitoring -- curl -m 5 -I http://192.168.0.45:9200 2>&1

echo
echo "2. Recent ES Logs (if accessible):"
curl -s "http://192.168.0.45:9200/_cluster/health"

echo
echo "3. Logstash Errors:"
kubectl logs deployment/logstash -n monitoring --tail=10 | grep -i error

echo
echo "4. Index Status:"
curl -s "http://192.168.0.45:9200/_cat/indices/aggregator-logs-*?h=index,health,status"

echo
echo "5. ILM Issues:"
curl -s "http://192.168.0.45:9200/_ilm/explain/aggregator-logs-*" | jq '.indices[] | select(.failed_step != null)'
```

### Index Cleanup and Maintenance
```bash
# Delete old indices (careful!)
curl -X DELETE "http://192.168.0.45:9200/aggregator-logs-2025.08.11"

# Reindex data (if needed)
curl -X POST "http://192.168.0.45:9200/_reindex?pretty" \
  -H "Content-Type: application/json" \
  -d '{
    "source": {"index": "old-index"},
    "dest": {"index": "aggregator-logs"}
  }'

# Close/open indices
curl -X POST "http://192.168.0.45:9200/aggregator-logs-000001/_close?pretty"
curl -X POST "http://192.168.0.45:9200/aggregator-logs-000001/_open?pretty"
```

---

## 8. Monitoring Automation

### Automated Health Monitoring
```bash
# Continuous cluster monitoring
watch -n 30 'curl -s "http://192.168.0.45:9200/_cluster/health" | jq ".status, .number_of_data_nodes, .active_shards"'

# Log ingestion rate monitoring
watch -n 10 'curl -s "http://192.168.0.45:9200/aggregator-logs/_count" | jq ".count"'

# ILM policy execution monitoring
watch -n 60 'curl -s "http://192.168.0.45:9200/_ilm/explain/aggregator-logs-*" | jq ".indices | to_entries[] | {index: .key, phase: .value.phase, action: .value.action}"'
```

### Alert Thresholds
```bash
# Check for yellow/red cluster status
curl -s "http://192.168.0.45:9200/_cluster/health" | jq -r '.status' | grep -v green && echo "ALERT: Cluster not green"

# Check for high heap usage
curl -s "http://192.168.0.45:9200/_nodes/stats/jvm" | jq '.nodes[] | .jvm.mem.heap_used_percent' | awk '$1 > 85 {print "ALERT: High heap usage: " $1 "%"}'

# Check for unassigned shards
curl -s "http://192.168.0.45:9200/_cluster/health" | jq '.unassigned_shards' | awk '$1 > 0 {print "ALERT: Unassigned shards: " $1}'
```

---

## 9. Integration with Kubernetes

### Kubernetes-Specific ELK Commands

#### Pod Resource Monitoring
```bash
# ELK pod resource usage
kubectl top pods -n monitoring | grep -E "(logstash|kibana)"

# Detailed pod information
kubectl describe pod -l app=logstash -n monitoring
kubectl describe pod -l app=kibana -n monitoring

# Pod events and issues
kubectl get events -n monitoring --field-selector involvedObject.name=logstash-xxx
```

#### Service and Networking
```bash
# Check ELK services
kubectl get svc -n monitoring | grep -E "(logstash|kibana)"

# Test internal service connectivity
kubectl exec deployment/logstash -n monitoring -- nslookup kibana.monitoring.svc.cluster.local

# Ingress status for Kibana
kubectl get ingress -n monitoring | grep kibana
kubectl describe ingress monitoring-ingress -n monitoring
```

### Configuration Management
```bash
# Backup current configurations
kubectl get configmap logstash-pipeline -n monitoring -o yaml > logstash-config-backup.yaml
kubectl get configmap kibana-config -n monitoring -o yaml > kibana-config-backup.yaml

# Apply configuration changes
kubectl apply -f updated-logstash-config.yaml
kubectl rollout restart deployment/logstash -n monitoring
```

---

## 10. Common Troubleshooting Scenarios

### Scenario 1: Connection Refused Errors
```bash
# Check ES service status
curl -I "http://192.168.0.45:9200"

# Test from Kubernetes
kubectl exec deployment/logstash -n monitoring -- curl -I http://192.168.0.45:9200

# Check Logstash configuration
kubectl exec deployment/logstash -n monitoring -- cat /usr/share/logstash/pipeline/logstash.conf | grep elasticsearch
```

### Scenario 2: High Memory Usage
```bash
# ES heap usage
curl "http://192.168.0.45:9200/_nodes/stats/jvm?filter_path=nodes.*.jvm.mem.heap_used_percent"

# Kubernetes pod memory
kubectl exec deployment/logstash -n monitoring -- cat /sys/fs/cgroup/memory.current
kubectl exec deployment/kibana -n monitoring -- cat /sys/fs/cgroup/memory.current
```

### Scenario 3: Index Management Issues
```bash
# Check failed ILM executions
curl "http://192.168.0.45:9200/_ilm/explain/aggregator-logs-*?only_errors&pretty"

# Force ILM execution
curl -X POST "http://192.168.0.45:9200/_ilm/_move/aggregator-logs-000002" \
  -H "Content-Type: application/json" \
  -d '{"current_step": {"phase": "hot", "action": "rollover", "name": "attempt-rollover"}}'

# Check alias configuration
curl "http://192.168.0.45:9200/_alias/aggregator-logs?pretty"
```

---

## Usage Examples

### Complete Health Check Pipeline
```bash
# 1. Test basic connectivity
curl -I http://192.168.0.45:9200

# 2. Check cluster health
curl "http://192.168.0.45:9200/_cluster/health?pretty"

# 3. Verify indices
curl "http://192.168.0.45:9200/_cat/indices/aggregator-logs-*?v"

# 4. Check ILM status
curl "http://192.168.0.45:9200/_ilm/status?pretty"

# 5. Test Kubernetes connectivity
kubectl exec deployment/logstash -n monitoring -- curl -I http://192.168.0.45:9200

# 6. Check recent logs
kubectl logs deployment/logstash -n monitoring --tail=10
```

### Performance Optimization Check
```bash
# 1. Check segment count (should be low after force merge)
curl "http://192.168.0.45:9200/_cat/segments/aggregator-logs-*?h=index,segment,size&s=size:desc"

# 2. Check cache hit rates
curl "http://192.168.0.45:9200/_nodes/stats/indices/query_cache?pretty"

# 3. Monitor search performance
curl "http://192.168.0.45:9200/_nodes/stats/indices/search?pretty"
```

This reference guide provides comprehensive API coverage for managing and troubleshooting your ELK stack configuration. Keep this document accessible for quick operational reference and troubleshooting procedures.