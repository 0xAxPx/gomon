# Simple Elasticsearch Cluster Verification Commands

echo "=== Quick ES Cluster Verification ==="

# ============================================================================
# Run these commands to verify your cluster is working
# ============================================================================

# 1. Check cluster health
echo "1. Cluster Health:"
curl -X GET "http://192.168.0.45:9200/_cluster/health?pretty"

echo ""
echo "2. List nodes in cluster:"
curl -X GET "http://192.168.0.45:9200/_cat/nodes?v"

echo ""
echo "3. Check from second node:"
curl -X GET "http://192.168.0.157:9200/_cluster/health?pretty"

echo ""
echo "4. Check both node versions:"
curl -s "http://192.168.0.45:9200" | grep '"number"'
curl -s "http://192.168.0.157:9200" | grep '"number"'

echo ""
echo "5. List current indices:"
curl -X GET "http://192.168.0.45:9200/_cat/indices?v"

echo ""
echo "=== What to Look For ==="
echo '✅ "status" : "green"'
echo '✅ "number_of_nodes" : 2'  
echo "✅ Both nodes listed: es-node-1 and es-node-2"
echo "✅ Both showing version 8.19.1"
