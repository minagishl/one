#!/bin/sh

# Fix permissions for mounted volumes
echo "Fixing permissions for /app/temp..."

# Check if running as root (for permission fixing)
if [ "$(id -u)" = "0" ]; then
    echo "Running as root, fixing permissions..."
    
    # Ensure temp directory exists with correct permissions
    mkdir -p /app/temp/files
    chown -R appuser:appuser /app/temp
    chmod -R 755 /app/temp
    
    echo "Permissions fixed, switching to appuser..."
    # Switch to appuser and execute the main application
    exec su-exec appuser "$@"
else
    echo "Running as non-root user ($(whoami)), attempting to fix permissions..."
    
    # Try to create directories and fix permissions as much as possible
    mkdir -p /app/temp/files 2>/dev/null || true
    chmod 755 /app/temp 2>/dev/null || true
    chmod 755 /app/temp/files 2>/dev/null || true
    
    # Execute the main application
    exec "$@"
fi