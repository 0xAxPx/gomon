#!/bin/bash
# GoMon Resource Analysis Script

echo "=== GoMon Infrastructure Resource Analysis ==="
echo "Date: $(date)"
echo

# 1. Current Resource Allocations
echo "1. CURRENT RESOURCE ALLOCATIONS:"
echo "================================"
kubectl get pods -n monitoring -o custom-columns="NAME:.metadata.name,CPU-REQ:.spec.containers[*].resources.requests.cpu,CPU-LIM:.spec.containers[*].resources.limits.cpu,MEM-REQ:.spec.containers[*].resources.requests.memory,MEM-LIM:.spec.containers[*].resources.limits.memory"

echo
echo "2. ACTUAL RESOURCE USAGE:"
echo "========================"
kubectl top pods -n monitoring --containers

echo
echo "3. NODE RESOURCE PRESSURE:"
echo "========================="
kubectl top nodes
kubectl describe nodes | grep -A 5 "Allocated resources"

echo
echo "4. DETAILED POD RESOURCE ANALYSIS:"
echo "=================================="
for pod in $(kubectl get pods -n monitoring -o jsonpath='{.items[*].metadata.name}'); do
    echo "--- Pod: $pod ---"
    
    # CPU and Memory limits from cgroups
    echo "Resource limits (cgroups):"
    kubectl exec $pod -n monitoring -- cat /sys/fs/cgroup/memory.max 2>/dev/null || echo "Memory limit: N/A"
    kubectl exec $pod -n monitoring -- cat /sys/fs/cgroup/cpu.max 2>/dev/null || echo "CPU limit: N/A"
    
    # Current usage
    echo "Current usage:"
    kubectl exec $pod -n monitoring -- cat /sys/fs/cgroup/memory.current 2>/dev/null || echo "Memory usage: N/A"
    kubectl exec $pod -n monitoring -- cat /proc/loadavg 2>/dev/null || echo "Load avg: N/A"
    
    echo
done

echo
echo "5. PERSISTENT VOLUME USAGE:"
echo "==========================="
kubectl get pvc -n monitoring -o custom-columns="NAME:.metadata.name,SIZE:.spec.resources.requests.storage,STATUS:.status.phase"

echo "Disk usage inside pods:"
for pod in $(kubectl get pods -n monitoring -o jsonpath='{.items[*].metadata.name}'); do
    echo "--- Pod: $pod ---"
    kubectl exec $pod -n monitoring -- df -h 2>/dev/null | head -10 || echo "Cannot access filesystem"
    echo
done

echo
echo "6. KAFKA CLUSTER RESOURCE ANALYSIS:"
echo "===================================="
for i in {0..2}; do
    echo "--- Kafka-$i ---"
    kubectl exec kafka-$i -n monitoring -- cat /proc/meminfo | grep -E "(MemTotal|MemAvailable|MemFree)" 2>/dev/null || echo "Memory info unavailable"
    kubectl exec kafka-$i -n monitoring -- du -sh /var/lib/kafka/data 2>/dev/null || echo "Disk usage unavailable"
done

echo
echo "7. ELASTICSEARCH EXTERNAL RESOURCE CHECK:"
echo "========================================="
curl -s "http://192.168.0.45:9200/_nodes/stats/jvm,os" | jq '.nodes[] | {name: .name, heap_used_percent: .jvm.mem.heap_used_percent, cpu_percent: .os.cpu.percent, memory_used_percent: .os.mem.used_percent}' 2>/dev/null || echo "ES metrics unavailable"

echo
echo "8. RESOURCE EFFICIENCY ANALYSIS:"
echo "==============================="
echo "Calculating CPU and Memory efficiency ratios..."
kubectl top pods -n monitoring --containers > /tmp/actual_usage.txt 2>/dev/null
kubectl get pods -n monitoring -o json | jq -r '.items[] | [.metadata.name, (.spec.containers[].resources.requests.cpu // "0"), (.spec.containers[].resources.requests.memory // "0")] | @tsv' > /tmp/requested_resources.txt

echo "Resource allocation vs usage comparison:"
echo "Pod Name | CPU Efficiency | Memory Efficiency"
echo "---------|----------------|------------------"
# This would need more complex processing to calculate actual ratios

echo
echo "=== ANALYSIS COMPLETE ==="
echo "Review the output above to identify:"
echo "1. Over-provisioned pods (low usage vs high limits)"
echo "2. Under-provisioned pods (high usage near limits)" 
echo "3. Storage growth patterns"
echo "4. Optimization opportunities"