#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=> Building binary..."
cd "$SCRIPT_DIR"
go build -o backup-service .

echo "=> Stopping existing process if running..."
sudo systemctl stop backup-service.service 2>/dev/null || true
pkill -f "$SCRIPT_DIR/backup-service" 2>/dev/null || true

echo "=> Installing systemd unit..."
sudo cp systemd/backup-service.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable backup-service.service
sudo systemctl start backup-service.service

echo "=> Checking status..."
sudo systemctl status backup-service.service --no-pager

echo "=> Done. Service running on port ${WEB_PORT:-8090}"
