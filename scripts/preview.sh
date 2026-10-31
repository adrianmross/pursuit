#!/usr/bin/env bash
# Hot-reloading preview of this worktree at https://pursuit-<branch>.localhost
# (portless names it from the worktree dir, like pursuit-dev). The dashboard
# runs under `next dev` with HMR; the Go API restarts on .go changes. Edit
# either side and the page updates — no static export rebuild.
#
# Data lives in .devdata/ (git-ignored); the first start prints a bootstrap
# token. Set PURSUIT_CONFIG to a different file if your config won't load.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
name="pursuit-$(basename "$PWD" | sed -n 's/^pursuit\.//p')"
[ "$name" = "pursuit-" ] && name="pursuit"
api_port="${PURSUIT_API_PORT:-$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')}"
export PURSUIT_DATA_DIR="${PURSUIT_DATA_DIR:-$PWD/.devdata}"
mkdir -p "$PURSUIT_DATA_DIR"

# The binary go:embeds the dashboard; next dev serves it here, so a stub
# is enough for the API to compile.
if [ ! -e internal/dashboard/dist/index.html ]; then
  mkdir -p internal/dashboard/dist
  echo 'served by next dev' > internal/dashboard/dist/index.html
fi

find . -name '*.go' -not -path './web-app/*' -not -path './docs-site/*' |
  PURSUIT_ADDR="127.0.0.1:$api_port" entr -rn go run ./cmd/pursuit serve &
api=$!
trap 'kill "$api" 2>/dev/null || true' EXIT

cd web-app
[ -d node_modules ] || npm install --no-audit --no-fund
echo "pursuit preview: https://$name.localhost (API on 127.0.0.1:$api_port)"
PURSUIT_API_URL="http://127.0.0.1:$api_port" portless "$name" npx next dev
