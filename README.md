

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



