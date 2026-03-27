#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "=> Building React frontend..."
cd frontend
npm run build
cd ..

echo "=> Copying frontend build to dist..."
rm -rf dist
cp -r frontend/dist dist

echo "=> Building Go backend..."
go build -o backup-service .

echo "=> Done!"
