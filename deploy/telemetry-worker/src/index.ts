// Receives the anonymous install ping from `pursuit serve` (see
// docs-site/content/telemetry.mdx) and answers with the latest release.
// Nothing is stored beyond the fields below; the client IP is never read.

interface Env {
  PINGS: AnalyticsEngineDataset
}

const MAX_BODY = 1024
const ID = /^[0-9a-f]{32}$/
const SHORT = /^[\w.+-]{1,32}$/

async function latestRelease(): Promise<string> {
  const res = await fetch("https://api.github.com/repos/adrianmross/pursuit/releases/latest", {
    headers: { "User-Agent": "pursuit-telemetry", Accept: "application/vnd.github+json" },
    cf: { cacheTtl: 3600, cacheEverything: true },
  })
  if (!res.ok) return ""
  const { tag_name } = (await res.json()) as { tag_name?: string }
  return (tag_name ?? "").replace(/^v/, "")
}

export default {
  async fetch(req: Request, env: Env): Promise<Response> {
    const { pathname } = new URL(req.url)
    if (req.method !== "POST" || pathname !== "/v1/ping") return new Response("not found", { status: 404 })

    const raw = await req.text()
    if (raw.length > MAX_BODY) return new Response("too large", { status: 413 })
    let p: Record<string, unknown>
    try {
      p = JSON.parse(raw)
    } catch {
      return new Response("bad json", { status: 400 })
    }
    const s = (k: string) => (typeof p[k] === "string" && SHORT.test(p[k] as string) ? (p[k] as string) : "")
    if (typeof p.install_id !== "string" || !ID.test(p.install_id)) return new Response("bad install_id", { status: 400 })

    env.PINGS.writeDataPoint({
      indexes: [p.install_id],
      blobs: [s("version"), s("os"), s("arch"), s("db"), p.docker === true ? "docker" : "host"],
      doubles: [1],
    })
    return Response.json({ latest: await latestRelease() })
  },
}
