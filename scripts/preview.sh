#!/usr/bin/env bash
# Hot-reloading preview of this worktree: `next dev` for the dashboard (HMR) in
# front of the Go API, which restarts on .go changes. Portless names it
# https://<branch>.pursuit.localhost in a worktree, https://pursuit.localhost in
# the primary checkout (the name comes from web-app/portless.json).
#
#   pursuit-preview          run in the foreground (Ctrl-C stops it)
#   pursuit-preview up       run detached, wait until it answers, print the URL
#   pursuit-preview down     stop this worktree's preview
#   pursuit-preview status   this worktree's state, then every preview on the machine
#   pursuit-preview logs     follow this worktree's log
#
# up/down/status/logs are scripts/devserver.sh, which works for any dev server.
# Each worktree has its own name, API port, data dir and log, so several previews
# run at once. Data lives in .devdata/ (git-ignored); the first start prints a
# bootstrap token. Set PURSUIT_CONFIG to a different file if your config won't load.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

run() {
  export PURSUIT_DATA_DIR="${PURSUIT_DATA_DIR:-$PWD/.devdata}"
  mkdir -p "$PURSUIT_DATA_DIR"
  api_port="${PURSUIT_API_PORT:-$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')}"

  # The binary go:embeds the dashboard; next dev serves it here, so a stub
  # is enough for the API to compile.
  if [ ! -e internal/dashboard/dist/index.html ]; then
    mkdir -p internal/dashboard/dist
    echo 'served by next dev' >internal/dashboard/dist/index.html
  fi

  find . -name '*.go' -not -path './web-app/*' -not -path './docs-site/*' |
    PURSUIT_ADDR="127.0.0.1:$api_port" entr -rn go run ./cmd/pursuit serve &
  api=$!
  trap 'kill "$api" 2>/dev/null || true' EXIT

  # The dashboard's URL is printed only once the API answers, so "up" means both halves.
  for _ in $(seq 1 90); do
    curl -sf -o /dev/null "http://127.0.0.1:$api_port/healthz" && break
    sleep 2
  done

  cd web-app
  [ -d node_modules ] || npm install --no-audit --no-fund
  PURSUIT_API_URL="http://127.0.0.1:$api_port" portless run npx next dev
}

case "${1:-}" in
"" | run) run ;;
up) exec bash scripts/devserver.sh up -- bash scripts/preview.sh run ;;
down | status | logs) exec bash scripts/devserver.sh "$1" ;;
*)
  echo "usage: pursuit-preview [up|down|status|logs]" >&2
  exit 2
  ;;
esac
