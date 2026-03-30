#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "=> Building Go API backend..."
go build -o backup-service .

echo "=> Done!"

echo "=> Starting server..."
exec ./backup-service
