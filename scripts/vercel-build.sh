#!/bin/sh
# Builds the dashboard and docs into internal/dashboard/dist for the Go
# function to embed. Lives in a script because vercel.json caps buildCommand
# at 256 characters.
set -e
(cd web && npm install && npm run build)
rm -rf internal/dashboard/dist
cp -r web/dist internal/dashboard/dist
(cd docs-site && npm install && npm run build)
cp -r docs-site/out internal/dashboard/dist/docs
mkdir -p public
echo 'served by api/index.go' > public/.keep
