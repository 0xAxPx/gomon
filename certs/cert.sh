#!/bin/bash

set -e

# Create cert directory if it doesn't exist
mkdir -p certs
cd certs

# Generate CA if not present
if [[ ! -f ca-cert.pem || ! -f ca-key.pem ]]; then
  echo "🔧 Generating CA (ca-cert.pem and ca-key.pem)..."
  openssl genrsa -out ca-key.pem 4096
  openssl req -x509 -new -nodes -key ca-key.pem -sha256 -days 365 \
    -subj "/CN=Kafka-CA" \
    -out ca-cert.pem
  echo "✅ CA generated"
else
  echo "✅ CA already exists"
fi

# Broker IDs
BROKER_IDS=(0 1 2)
HEADLESS_SERVICE="kafka-headless"
NAMESPACE="monitoring"

for BROKER_ID in "${BROKER_IDS[@]}"; do
  BROKER_DNS="kafka-${BROKER_ID}.${HEADLESS_SERVICE}.${NAMESPACE}.svc.cluster.local"
  
  echo "🔧 Generating cert for ${BROKER_DNS}..."

  # Private key
  openssl genrsa -out kafka-${BROKER_ID}-key.pem 2048

  # CSR with SAN
  openssl req -new -key kafka-${BROKER_ID}-key.pem -out kafka-${BROKER_ID}.csr \
    -subj "/CN=${BROKER_DNS}" \
    -addext "subjectAltName=DNS:${BROKER_DNS}"

  # Sign with CA
  openssl x509 -req \
    -in kafka-${BROKER_ID}.csr \
    -CA ca-cert.pem \
    -CAkey ca-key.pem \
    -CAcreateserial \
    -out kafka-${BROKER_ID}-cert.pem \
    -days 365 -sha256 \
    -extfile <(echo "subjectAltName=DNS:${BROKER_DNS}")

  echo "✅ Created: kafka-${BROKER_ID}-cert.pem and kafka-${BROKER_ID}-key.pem"
done

echo "🎉 All Kafka broker certificates and CA generated successfully."
