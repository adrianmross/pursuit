#!/bin/sh
# Builds the dashboard and docs into internal/dashboard/dist for the Go
# function to embed. Lives in a script because vercel.json caps buildCommand
# at 256 characters.
#
# web-app builds in its static-export mode (self-hosted) here, same as a
# self-hosted `pursuit serve` embeds it — this keeps today's deploy shape
# (one Go function serving everything) working as-is with the new Next.js
# dashboard. Running web-app as a real Next.js server on Vercel, for true
# SSR/streaming, is a separate, bigger reconfiguration (routing split,
# Vercel project settings) — not done by this script.
set -e
(cd web-app && npm install && npm run build:self-hosted)
rm -rf internal/dashboard/dist
cp -r web-app/out internal/dashboard/dist
(cd docs-site && npm install && npm run build)
cp -r docs-site/out internal/dashboard/dist/docs
mkdir -p public
echo 'served by api/index.go' > public/.keep
