#!/bin/sh
# Gets a fresh self-hosted server ready for the 0-8 storyboard: the example data
# the Tailored documents feature on, and two résumé versions, the second one on the first application.
#
#   PURSUIT_TELEMETRY=off PURSUIT_PREMIUM_FEATURES=on PURSUIT_FIXED_TIME=2026-11-08T15:00:00Z \
#     pursuit serve --addr 127.0.0.1:53998 --database-url sqlite:/tmp/reel8.db \
#     --owner-name "Dana Voss" --owner-email dana@example.com --bootstrap-token pst_reelfixture
#   sh docs-site/reel/prepare-0-8.sh http://127.0.0.1:53998 pst_reelfixture
#   reel build docs-site/reel/0-8.json
set -eu
BASE=${1:?server URL}
TOKEN=${2:?owner token}
T=$(mktemp -d)
trap 'rm -rf "$T"' EXIT
H="Authorization: Bearer $TOKEN"
curl -fsS -X POST "$BASE/v1/import/seed" -H "$H" -H 'Content-Type: application/json' -d '{}' >/dev/null
cat >"$T/main.md" <<'M'
# Sam Rivera

Senior software engineer, 8 years building web platforms.

## Experience

Acme Cloud, Staff Engineer. Led a team of six moving billing onto event streams; cut invoice errors by 40%.
Brightside, Senior Engineer. Built the public API and its docs; mentored four engineers.

## Skills

Go, TypeScript, PostgreSQL, Kubernetes.
M
cat >"$T/platform.md" <<'M'
# Sam Rivera

Senior software engineer, 8 years building developer platforms and APIs.

## Experience

Acme Cloud, Staff Engineer. Led a team of six on the platform behind billing; designed the public API surface and cut invoice errors by 40%.
Brightside, Senior Engineer. Built the public API and its docs; mentored four engineers and ran the on-call rotation.

## Skills

Go, TypeScript, PostgreSQL, Kubernetes, API design.
M
curl -fsS -X PATCH "$BASE/v1/features/tailored_documents" -H "$H" -H 'Content-Type: application/json' -d '{"enabled":true}' >/dev/null
P=${PURSUIT:-pursuit}
export PURSUIT_SERVER=$BASE PURSUIT_TOKEN=$TOKEN
$P documents add --kind base_resume --file "$T/main.md" --label "Main résumé" >/dev/null
DOC=$($P documents add --kind base_resume --file "$T/platform.md" --label "Platform roles" --json | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])')
APP=$(curl -fsS "$BASE/v1/applications?limit=1" -H "$H" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"][0]["id"])')
curl -fsS -X POST "$BASE/v1/applications/$APP/documents" -H "$H" -H 'Content-Type: application/json' -d "{\"document_id\":\"$DOC\"}" >/dev/null
echo "ready: application $APP uses Platform roles"
