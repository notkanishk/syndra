#!/bin/bash
set -e

echo "Updating MkAuth Stack..."

# Update code from repository (Assuming github deployment)
git pull origin main

# Rebuild containers and restart them
docker compose build
docker compose up -d

# Clean up unused, dangling images left behind
docker image prune -f

echo "Update complete! MkAuth stack has been restarted."
