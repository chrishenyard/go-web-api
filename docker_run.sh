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
docker compose up --force-recreate --remove-orphans -d

echo "Deployment successful!"
