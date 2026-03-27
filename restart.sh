#!/bin/bash
cd "$(dirname "$0")"

if [ "${DEV_MODE:-0}" = "1" ]; then
    pkill -f "backup-service" 2>/dev/null || true
    sleep 1
    go build -o backup-service . && ./backup-service
else
    sudo systemctl restart backup-service
fi
