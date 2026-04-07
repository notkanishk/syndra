#!/bin/bash
set -e

echo "Starting MkAuth Installation for Proxmox LXC..."

# Ensure strictly required dependencies
if ! command -v docker &> /dev/null; then
    echo "Docker is not installed. Installing Docker..."
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    rm get-docker.sh
fi

echo "Pulling Docker Images and starting MkAuth stack..."
docker compose pull
docker compose up -d --build

echo "Installation complete! MkAuth is running."
echo "UI is accessible at http://<LXC_IP>:3000"
echo "API is accessible at http://<LXC_IP>:8080"
