# Telemetry Worker

Receives the anonymous ping from `pursuit serve` and answers `{"latest": "<version>"}`. Pings go to a Workers Analytics Engine dataset (`pursuit_pings`): index = install id, blobs = version, os, arch, db, docker|host.

```bash
npx wrangler deploy
```

Run it once from this directory in your Cloudflare account. It serves from `pursuit-telemetry.winter-tooth-73af.workers.dev`, because `hirepursuit.app` is not on Cloudflare DNS. Released binaries bake that URL in, so keep the Worker name and account subdomain. If you move to a custom domain later, keep this URL answering for older releases.

Active installs in the last 30 days, by version (Analytics Engine SQL API):

```sql
SELECT blob1 AS version, COUNT(DISTINCT index1) AS installs
FROM pursuit_pings
WHERE timestamp > NOW() - INTERVAL '30' DAY
GROUP BY version
ORDER BY installs DESC
```
