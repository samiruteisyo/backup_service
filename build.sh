#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "=> Building..."
go build -o backup-service .


