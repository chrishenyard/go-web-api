#!/bin/bash

# 1. Exit immediately if any command fails
set -e

# 2. Check if .env file exists, then source (load) the variables
if [ -f .env ]; then
    echo "Loading environment variables from .env..."
    source .env
else
    echo "Error: .env file not found!"
    exit 1
fi

# 3. Build the Docker image
echo "Building Docker image..."
docker build -t go-web-api:latest .

# 4. Run the Docker container using the loaded variables
echo "Starting container on port $PORT..."
docker run -d \
  -p 9000:9000 \
  -e VAULT_ADDR="$VAULT_ADDR" \
  -e VAULT_KUBERNETES_JWT_PATH="$VAULT_KUBERNETES_JWT_PATH" \
  -e VAULT_MOUNT_POINT="$VAULT_MOUNT_POINT" \
  -e VAULT_ROLE_ID="$VAULT_ROLE_ID" \
  -e VAULT_ROLE_NAME="$VAULT_ROLE_NAME" \
  -e VAULT_SECRET_ID="$VAULT_SECRET_ID" \
  -e VAULT_SECRET_PATH="$VAULT_SECRET_PATH" \
  --add-host=vault.localhost:host-gateway \
  --name go-web-api \
  go-web-api:latest

echo "Deployment successful!"
