# Telemetry Worker

Receives the anonymous ping from `pursuit serve` and answers `{"latest": "<version>"}`. Pings go to a Workers Analytics Engine dataset (`pursuit_pings`): index = install id, blobs = version, os, arch, db, docker|host.

```bash
npx wrangler deploy
```

Run it once from this directory in your Cloudflare account. `telemetry.hirepursuit.app` must be a zone you control.

Active installs in the last 30 days, by version (Analytics Engine SQL API):

```sql
SELECT blob1 AS version, COUNT(DISTINCT index1) AS installs
FROM pursuit_pings
WHERE timestamp > NOW() - INTERVAL '30' DAY
GROUP BY version
ORDER BY installs DESC
```
