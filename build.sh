#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "=> Building..."
go build -o backup-service .

echo "=> Restarting service..."
sudo systemctl restart backup-service.service

echo "=> Status:"
sudo systemctl status backup-service.service --no-pager
