// Client-side API access. Two modes, picked at build time:
//
// - Hosted (default): calls go through /api/proxy/*, which reads the
//   httpOnly session cookie server-side. The browser never holds a token.
// - Self-hosted (NEXT_PUBLIC_EXPORT_MODE=1, static export): there is no
//   Next.js server at runtime — the Go binary just serves static files — so
//   there's nothing to proxy through. The browser calls the Go API directly
//   with a bearer token it holds itself, same as the old Vite app did.
"use client"

const EXPORT_MODE = process.env.NEXT_PUBLIC_EXPORT_MODE === "1"
const PURSUIT_API_URL = process.env.NEXT_PUBLIC_PURSUIT_API_URL || "http://127.0.0.1:8787"
const TOKEN_KEY = "pursuit.token"

export class ApiError extends Error {
  status: number
  hint?: string
  constructor(message: string, status: number, hint?: string) {
    super(message)
    this.status = status
    this.hint = hint
  }
}

// Only meaningful in export mode — the hosted build never reads this.
export function getToken() {
  if (typeof window === "undefined") return ""
  return localStorage.getItem(TOKEN_KEY) || ""
}

export function setToken(token: string) {
  if (typeof window === "undefined") return
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

type ApiOptions = {
  method?: string
  body?: unknown
  rawBody?: { text: string; contentType: string }
}

export async function api<T>(path: string, options: ApiOptions = {}): Promise<T> {
  const headers: Record<string, string> = {}
  const init: RequestInit = { method: options.method || "GET", headers }
  if (options.rawBody) {
    headers["Content-Type"] = options.rawBody.contentType
    init.body = options.rawBody.text
  } else if (options.body !== undefined) {
    headers["Content-Type"] = "application/json"
    init.body = JSON.stringify(options.body)
  }

  let url: string
  if (EXPORT_MODE) {
    url = PURSUIT_API_URL + path
    const token = getToken()
    if (token) headers.Authorization = "Bearer " + token
  } else {
    url = "/api/proxy" + path
  }

  const response = await fetch(url, init)
  const text = await response.text()
  let data: { error?: { message?: string; hint?: string } } | null = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = { error: { message: text } }
    }
  }
  if (!response.ok) {
    const body = data?.error || {}
    throw new ApiError(body.message || response.statusText || "Request failed", response.status, body.hint)
  }
  return data as T
}

// For calls that can't go through api<T>() (binary responses, e.g. a
// favicon image) — same proxy-vs-direct branching, caller does its own fetch.
export function apiUrl(path: string): string {
  return EXPORT_MODE ? PURSUIT_API_URL + path : "/api/proxy" + path
}

export function apiHeaders(): Record<string, string> {
  if (!EXPORT_MODE) return {}
  const token = getToken()
  return token ? { Authorization: "Bearer " + token } : {}
}

export { EXPORT_MODE }
