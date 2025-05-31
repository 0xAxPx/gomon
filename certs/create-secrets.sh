#!/bin/bash

set -e

cd certs

NAMESPACE="monitoring"
BROKER_IDS=(0 1 2)

for BROKER_ID in "${BROKER_IDS[@]}"; do
  SECRET_NAME="kafka-${BROKER_ID}-tls"

  echo "🔐 Creating secret ${SECRET_NAME} in namespace ${NAMESPACE}..."

  kubectl create secret tls ${SECRET_NAME} \
    --cert=kafka-${BROKER_ID}-cert.pem \
    --key=kafka-${BROKER_ID}-key.pem \
    -n ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

  echo "✅ Secret ${SECRET_NAME} created or updated."
done

echo "🎉 All secrets applied."
