#!/bin/bash
# Server-side deployment script for gate149-frontend
# Run this on Ubuntu server after uploading the image archive

ARCHIVE_PATH="/tmp/gate149-frontend.tar.gz"
TAR_FILE="/tmp/gate149-frontend.tar"
COMPOSE_DIR="/opt/gate/infrastructure"
CURRENT_STEP=""

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up temporary files..."
    
    if [ -f "$TAR_FILE" ]; then
        rm -f "$TAR_FILE"
        echo "  Removed: $TAR_FILE"
    fi
    
    if [ -f "$ARCHIVE_PATH" ]; then
        rm -f "$ARCHIVE_PATH"
        echo "  Removed: $ARCHIVE_PATH"
    fi
}

# Error handler
error_handler() {
    echo ""
    echo "========================================"
    echo "ERROR: Deployment failed!"
    echo "========================================"
    echo ""
    echo "Failed at step: $CURRENT_STEP"
    echo ""
    
    case "$CURRENT_STEP" in
        "extract")
            echo "Problem: Could not extract the archive"
            echo ""
            echo "Possible causes:"
            echo "  - Archive is corrupted"
            echo "  - Disk is full"
            echo ""
            echo "How to fix:"
            echo "  1. Re-upload from Windows: .\\deploy.ps1 -StartFrom upload"
            echo "  2. Run again: /tmp/server-deploy.sh"
            ;;
        "load")
            echo "Problem: Could not load Docker image"
            echo ""
            echo "Possible causes:"
            echo "  - Image tar is corrupted"
            echo "  - Docker daemon not running"
            echo "  - Not enough disk space"
            echo ""
            echo "How to fix:"
            echo "  1. Check Docker: sudo systemctl status docker"
            echo "  2. Check disk space: df -h"
            echo "  3. Re-upload and try again"
            ;;
        "compose_dir")
            echo "Problem: Compose directory not found: $COMPOSE_DIR"
            echo ""
            echo "How to fix:"
            echo "  1. Create directory: mkdir -p $COMPOSE_DIR"
            echo "  2. Add your docker-compose.yaml there"
            echo "  3. Run again: /tmp/server-deploy.sh"
            ;;
        "stop")
            echo "Problem: Could not stop the old container"
            echo ""
            echo "How to fix:"
            echo "  1. Check containers: docker ps -a"
            echo "  2. Stop manually: docker stop <name>"
            echo "  3. Run again: /tmp/server-deploy.sh"
            ;;
        "start")
            echo "Problem: Could not start the new container"
            echo ""
            echo "How to fix:"
            echo "  1. Check compose file: cat $COMPOSE_DIR/docker-compose.yaml"
            echo "  2. Check logs: docker compose logs"
            echo "  3. Try manually: cd $COMPOSE_DIR && docker compose up -d frontend"
            ;;
        "nginx")
            echo "Problem: Could not restart nginx"
            echo ""
            echo "How to fix:"
            echo "  1. Check nginx container: docker ps | grep nginx"
            echo "  2. Restart manually: cd $COMPOSE_DIR && docker compose restart nginx"
            ;;
        *)
            echo "Unknown error occurred"
            ;;
    esac
    
    echo ""
    cleanup
    exit 1
}

# Set trap for errors
trap error_handler ERR

echo "========================================"
echo "Gate149 Frontend Deployment"
echo "========================================"
echo ""

# Step 1: Extract archive
CURRENT_STEP="extract"
echo "[1/7] Extracting archive..."
if [ ! -f "$ARCHIVE_PATH" ]; then
    echo "Error: Archive not found at $ARCHIVE_PATH"
    echo ""
    echo "Upload the archive from Windows first:"
    echo "  .\\deploy.ps1 -StartFrom upload"
    exit 1
fi

gunzip "$ARCHIVE_PATH"
echo "OK - Archive extracted"
echo ""

# Step 2: Load Docker image
CURRENT_STEP="load"
echo "[2/7] Loading Docker image..."
docker load -i "$TAR_FILE"
echo "OK - Image loaded"
echo ""

# Step 3: Clean up tar file
echo "[3/7] Cleaning up archive..."
rm -f "$TAR_FILE"
echo "OK - Archive removed"
echo ""

# Step 4: Navigate to compose directory
CURRENT_STEP="compose_dir"
echo "[4/7] Navigating to compose directory..."
if [ ! -d "$COMPOSE_DIR" ]; then
    echo "Error: Directory $COMPOSE_DIR not found"
    error_handler
fi

cd "$COMPOSE_DIR"
echo "OK - Changed to $COMPOSE_DIR"
echo ""

# Step 5: Stop and remove old container
CURRENT_STEP="stop"
echo "[5/7] Stopping and removing old container..."

# Try to find the frontend service name
FRONTEND_SERVICE=$(docker compose ps --services 2>/dev/null | grep -E "frontend|gate149-frontend" | head -1)

if [ -z "$FRONTEND_SERVICE" ]; then
    echo "Warning: Could not detect frontend service name, using 'frontend'"
    FRONTEND_SERVICE="frontend"
fi

echo "Using service name: $FRONTEND_SERVICE"

# Stop container (ignore errors if not running)
docker compose stop "$FRONTEND_SERVICE" 2>/dev/null || echo "Container was not running"

# Remove container (ignore errors if doesn't exist)
docker compose rm -f "$FRONTEND_SERVICE" 2>/dev/null || echo "Container did not exist"

echo "OK - Old container removed"
echo ""

# Step 6: Start new container
CURRENT_STEP="start"
echo "[6/7] Starting new container..."
cd "$COMPOSE_DIR"
docker compose up -d "$FRONTEND_SERVICE"
echo "OK - Container started"
echo ""

# Step 7: Restart nginx
CURRENT_STEP="nginx"
echo "[7/7] Restarting nginx..."
cd "$COMPOSE_DIR"
docker compose restart nginx 2>/dev/null || echo "Warning: nginx not found or failed to restart"
echo "OK - Nginx restarted"
echo ""

# Show status
echo "========================================"
echo "Deployment Complete!"
echo "========================================"
echo ""
echo "Container status:"
docker compose ps "$FRONTEND_SERVICE"
docker compose ps nginx 2>/dev/null || true
echo ""
echo "View logs:"
echo "  docker compose logs -f $FRONTEND_SERVICE"
echo ""
