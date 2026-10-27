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

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ""
}

export function setToken(token: string) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

type ApiOptions = {
  method?: string
  body?: unknown
  retried?: boolean
}

export async function api<T>(path: string, options: ApiOptions = {}): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {}
  if (token) headers.Authorization = "Bearer " + token
  const init: RequestInit = { method: options.method || "GET", headers }
  if (options.body !== undefined) {
    headers["Content-Type"] = "application/json"
    init.body = JSON.stringify(options.body)
  }
  const response = await fetch(path, init)
  const text = await response.text()
  let data: { error?: { message?: string; hint?: string } } | null = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = { error: { message: text } }
    }
  }
  if (response.status === 401) {
    const hadToken = token !== ""
    setToken("")
    if (hadToken && !options.retried) {
      return api(path, { ...options, retried: true })
    }
    throw new ApiError("That token was rejected.", 401)
  }
  if (!response.ok) {
    const body = data?.error || {}
    throw new ApiError(body.message || response.statusText || "Request failed", response.status, body.hint)
  }
  return data as T
}
