#!/usr/bin/env bash
# Run a dev server detached, one per git worktree, and wait until its URL answers.
# Repo-agnostic: copy it to any project, or put it on your PATH. Portless does the
# naming (`portless run next dev` is https://<branch>.<project>.localhost in a
# worktree), so this only handles the lifecycle portless leaves to you.
#
#   devserver.sh up -- portless run next dev    start (a no-op if already up), print the URL
#   devserver.sh down                           stop this worktree's server
#   devserver.sh status                         this worktree's state, then every portless route
#   devserver.sh logs                           follow the log
#
# State (pid, log) lives in this worktree's own git dir, so nothing to ignore and
# nothing shared between worktrees. The URL is the first https://….localhost the
# command prints, which is what portless prints on start.
set -euo pipefail

state="$(git rev-parse --absolute-git-dir)/devserver"
mkdir -p "$state"
pidfile="$state/pid"
logfile="$state/log"
timeout="${DEVSERVER_TIMEOUT:-180}"

alive() { [ -f "$pidfile" ] && kill -0 "$(cat "$pidfile")" 2>/dev/null; }
url() { grep -o 'https://[A-Za-z0-9.-]*\.localhost[^[:space:]]*' "$logfile" 2>/dev/null | head -1 || true; }

cmd="${1:-}"
[ $# -gt 0 ] && shift
case "$cmd" in
up)
  [ "${1:-}" = "--" ] && shift
  [ $# -gt 0 ] || { echo "usage: devserver.sh up -- <command...>" >&2; exit 2; }
  if alive; then
    echo "already up: $(url)"
    exit 0
  fi
  : >"$logfile"
  # Its own session: the server outlives the shell, or agent tool call, that
  # started it. The pid is also the process group, which `down` signals.
  python3 -c 'import os,sys; os.setsid(); os.execvp(sys.argv[1], sys.argv[1:])' \
    "$@" >>"$logfile" 2>&1 </dev/null &
  echo $! >"$pidfile"
  for _ in $(seq 1 $((timeout / 2))); do
    alive || {
      echo "exited; last log lines:" >&2
      tail -20 "$logfile" >&2
      exit 1
    }
    u="$(url)"
    # Any HTTP status counts: a 404 or a redirect to sign-in still means it is serving.
    if [ -n "$u" ] && [ "$(curl -sk -o /dev/null -w '%{http_code}' "$u")" != 000 ]; then
      echo "$u"
      exit 0
    fi
    sleep 2
  done
  echo "no answer after ${timeout}s; see $logfile" >&2
  exit 1
  ;;
down)
  if alive; then
    kill -TERM -- "-$(cat "$pidfile")" 2>/dev/null || kill -TERM "$(cat "$pidfile")"
    echo "stopped $(url)"
  else
    echo "not running"
  fi
  rm -f "$pidfile"
  ;;
status)
  if alive; then echo "this worktree: up, $(url)"; else echo "this worktree: down"; fi
  portless list
  ;;
logs) tail -f "$logfile" ;;
*)
  echo "usage: devserver.sh up -- <command...> | down | status | logs" >&2
  exit 2
  ;;
esac
