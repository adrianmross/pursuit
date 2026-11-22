#!/bin/sh
# Gets a fresh self-hosted server ready for the 0-10 storyboards: the example
# data, a Claude app registered over OAuth and approved, so Settings, Connected
# apps has a row. Run it once per build, on an empty database: every API token
# a build creates stays in the list the next build films.
#
#   PURSUIT_TELEMETRY=off PURSUIT_FIXED_TIME=2026-11-09T15:00:00Z \
#     pursuit serve --addr 127.0.0.1:53998 --database-url sqlite:/tmp/reel.db \
#     --owner-name "Dana Voss" --owner-email dana@example.com \
#     --bootstrap-token pst_reelfixture
#   sh docs-site/reel/prepare.sh http://127.0.0.1:53998 pst_reelfixture
#   reel build docs-site/reel/0-10-consent.json
#   reel build docs-site/reel/0-10.json
#
# The Claude app's client id is random, so this rewrites it in the consent
# storyboard.
set -eu

BASE=${1:?server URL, such as http://127.0.0.1:53998}
TOKEN=${2:?owner token}
DIR=$(cd "$(dirname "$0")" && pwd)
JAR=$(mktemp)
trap 'rm -f "$JAR"' EXIT

curl -fsS -X POST "$BASE/v1/import/seed" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{}' >/dev/null

CLIENT=$(curl -fsS -X POST "$BASE/v1/auth/register" -H 'Content-Type: application/json' \
  -d "{\"client_name\":\"Claude\",\"redirect_uris\":[\"$BASE/\"],\"token_endpoint_auth_method\":\"none\",\"grant_types\":[\"authorization_code\",\"refresh_token\"],\"response_types\":[\"code\"],\"scope\":\"read write offline_access\"}" |
  python3 -c 'import sys,json;print(json.load(sys.stdin)["client_id"])')

VERIFIER=reelverifier0123456789abcdefghijklmnopqrstuvwxyz-ABCDEFG
CHALLENGE=$(python3 -c "import hashlib,base64;print(base64.urlsafe_b64encode(hashlib.sha256(b'$VERIFIER').digest()).decode().rstrip('='))")

curl -fsS -c "$JAR" -X POST "$BASE/v1/auth/session" -H 'Content-Type: application/json' -H "Origin: $BASE" -d "{\"token\":\"$TOKEN\"}" >/dev/null
CODE=$(curl -fsS -b "$JAR" -X POST "$BASE/v1/oauth/authorize" -H 'Content-Type: application/json' -H "Origin: $BASE" \
  -d "{\"client_id\":\"$CLIENT\",\"redirect_uri\":\"$BASE/\",\"code_challenge\":\"$CHALLENGE\",\"code_challenge_method\":\"S256\",\"state\":\"reel\",\"scope\":\"read write offline_access\",\"approve\":true}" |
  python3 -c 'import sys,json,urllib.parse as u;print(u.parse_qs(u.urlparse(json.load(sys.stdin)["redirect_to"]).query)["code"][0])')
curl -fsS -X POST "$BASE/v1/auth/token" \
  -d "grant_type=authorization_code&code=$CODE&client_id=$CLIENT&code_verifier=$VERIFIER" \
  --data-urlencode "redirect_uri=$BASE/" >/dev/null

python3 - "$DIR/0-10-consent.json" "$CLIENT" <<'PY'
import re, sys
path, client = sys.argv[1:]
text = open(path).read()
open(path, "w").write(re.sub(r"client_id=pc_[A-Za-z0-9_-]+", "client_id=" + client, text))
PY
echo "ready: Claude is registered and connected as $CLIENT"
