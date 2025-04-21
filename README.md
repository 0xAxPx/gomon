

1) Dockerfile.agent - docker file for agent service
2) Dockerfile.aggregator

# Set up docker network so that services could communicate
docker network create metrics-network

alex@Alexs-MBP gomon % docker network ls | grep metrics-network
2f4c697c06b5   metrics-network   bridge    local

# Pull Zookeeper Image
docker pull bitnami/zookeeper:latest

# Start Zookeeper as container
docker run -d \
  --name zookeeper \
  --network metrics-network \
  -e ALLOW_ANONYMOUS_LOGIN=yes \
  bitnami/zookeeper:latest

 # Pull Kafka Image 
 docker pull bitnami/kafka:latest

 # Start Kafka
docker run -d \
  --name kafka \
  --network metrics-network \
  -e KAFKA_CFG_ZOOKEEPER_CONNECT=zookeeper:2181 \
  -e KAFKA_CFG_LISTENERS=PLAINTEXT://:9092 \
  -e KAFKA_CFG_ADVERTISED_LISTENERS=PLAINTEXT://kafka:9092 \
  bitnami/kafka:latest

# Create Kafka Topic
@efa859e76970:/$ kafka-topics.sh --bootstrap-server kafka:9092 --create --topic metrics-topic --partitions 1 --replication-factor 1
Created topic metrics-topic.
@efa859e76970:/$ kafka-topics.sh --bootstrap-server kafka:9092 --list
metrics-topic

# Check Kafka messages
docker exec -it kafka kafka-console-consumer.sh --bootstrap-server kafka:9092 --topic metrics-topic --from-beginning

# Start Agent Container
docker build -t gomon-agent --no-cache -f Dockerfile.agent .
docker run -d --name gomon-agent-container --network metrics-network gomon-agent

# Start Aggregator Container
docker build -t aggregator --no-cache -f Dockerfile.aggregator .
docker run -d --name gomon-agg-container --network metrics-network aggregator

# Kubernetes
kubectl create namespace monitoring
kubectl get namespace
kubectl config set-context --current --namespace=monitoring

## Run Zookeper
kubectl apply -f k8s/zookeeper-deployment.yaml
kubectl logs -f `kubectl get pods | grep zookeeper | awk '{print $1}'`
or inside of pod
kubectl exec --it `kubect get deployment | grep zookeeper | awk '{print $1}'`

## Set up Kub dashboard
kubect get pods -A | grep dashboard
kubectl apply -f k8s/admin-user.yaml
ubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml
### Create token
kubectl -n monitoring create token admin-user
### Get proxy up
kubectl proxy
### Open browser and appy token 
http://localhost:8001/api/v1/namespaces/monitoring/services/https:monitoring:/proxy/


# Victoria Metrics
kubectl apply -f k8s/vm-deployment.yaml
kubectl port-forward -n monitoring svc/victoria-metrics 8428:8428

# Grafana
kubectl apply -f k8s/grafana-deployment.yaml

# Utility for Deployment & Services
kubectl delete deployments --all --all-namespaces
kubectl delete statefulsets --all --all-namespaces
kubectl delete replicasets --all --all-namespaces
kubectl delete daemonsets --all --all-namespaces

kubectl describe statefulset kafka -n monitoring






