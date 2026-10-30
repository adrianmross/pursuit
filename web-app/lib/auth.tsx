"use client"

import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react"
import { api, ApiError, EXPORT_MODE, setToken } from "@/lib/api-client"
import type { Me } from "@/lib/types"

type AuthState = {
  me: Me | null
  // Only meaningful in export mode: true until the mount-time /v1/me check
  // (below) resolves one way or the other. Hosted mode has nothing to wait
  // for — `me` is already server-seeded — so this is always false there.
  loading: boolean
  error: string | null
  signIn: (token: string) => Promise<void>
  // "Try it without an account" (signed out) and "replay tutorial"
  // (Settings, any time, signed in or not): mints a fresh throwaway
  // sandbox tenant and signs into it, same as a normal sign-in. 404s if
  // the deployment doesn't have demo mode on (see isDemoAvailable below).
  startDemo: () => Promise<void>
  signOut: () => Promise<void>
  // Re-reads /v1/me after the profile (name, avatar) changes.
  refresh: () => Promise<void>
  can: (scope: string) => boolean
  // True once signed into a demo sandbox (POST /v1/demo names its token
  // "demo" — see Service.CreateDemoSession). Settings uses this to label
  // its action "Replay tutorial" instead of "Try the tutorial".
  isDemo: boolean
}

const AuthContext = createContext<AuthState | null>(null)

// On the hosted build, `me` is fetched server-side (middleware already
// redirects anonymous requests to /sign-in, and the root layout passes the
// server-fetched identity down) — this provider just holds it in context.
// In export mode there's no server fetch to seed it from: it starts null,
// SignIn populates it directly after a successful token check, and — since
// a full page load/reload re-mounts this provider with initialMe back to
// null even though the browser still holds a valid token — the effect
// below re-populates it from that stored token on mount. Without this,
// `can()` silently goes back to false after any reload (the exact bug
// that made row actions vanish post-reload: see AppGate, which relies on
// this same state instead of doing its own disconnected check).
//
// The check always fires in export mode, even with no stored token — it
// just 401s harmlessly when there's genuinely nothing stored. Demo mode
// doesn't change this: a demo session is a normal token from
// POST /v1/demo (see startDemo below), stored exactly like a real one.
export function AuthProvider({ initialMe, children }: { initialMe: Me | null; children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(initialMe)
  const [loading, setLoading] = useState(EXPORT_MODE && initialMe == null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!EXPORT_MODE || me != null) {
      // Nothing to check (hosted, or already resolved) — settles the
      // one-time mount-time loading state, not a render loop.
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setLoading(false)
      return
    }
    let cancelled = false
    api<Me>("/v1/me")
      .then((result) => {
        if (!cancelled) setMe(result)
      })
      .catch(() => {
        if (!cancelled) setToken("")
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const signIn = useCallback(async (token: string) => {
    setError(null)
    try {
      if (EXPORT_MODE) {
        setToken(token)
        const result = await api<Me>("/v1/me")
        setMe(result)
      } else {
        const response = await fetch("/api/auth/sign-in", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token }),
        })
        const data = await response.json()
        if (!response.ok) throw new Error(data?.error?.message || "That token was rejected.")
        setMe(data as Me)
      }
    } catch (err) {
      setMe(null)
      setError(err instanceof ApiError || err instanceof Error ? err.message : "Sign-in failed")
      throw err
    }
  }, [])

  const startDemo = useCallback(async () => {
    setError(null)
    try {
      if (EXPORT_MODE) {
        const { token } = await api<{ token: string }>("/v1/demo", { method: "POST" })
        setToken(token)
        const result = await api<Me>("/v1/me")
        setMe(result)
      } else {
        const response = await fetch("/api/auth/demo", { method: "POST" })
        const data = await response.json()
        if (!response.ok) throw new Error(data?.error?.message || "Could not start the demo.")
        setMe(data as Me)
      }
    } catch (err) {
      setError(err instanceof ApiError || err instanceof Error ? err.message : "Could not start the demo.")
      throw err
    }
  }, [])

  const signOut = useCallback(async () => {
    if (EXPORT_MODE) {
      setToken("")
    } else {
      // Must await: navigating away can cancel an in-flight fire-and-forget
      // request before the server clears the cookie, leaving the user
      // still signed in after landing on /sign-in.
      await fetch("/api/auth/sign-out", { method: "POST" })
    }
    setMe(null)
    // A hard navigation, not router.push: forces every cached RSC payload
    // to be re-fetched so no stale authenticated state can linger client-side.
    // eslint-disable-next-line @next/next/no-location-assign-relative-destination
    window.location.assign("/sign-in")
  }, [])

  const can = useCallback(
    (scope: string) => {
      const scopes = me?.token.scopes || []
      return scopes.includes(scope) || (scope === "read" && scopes.includes("write"))
    },
    [me],
  )

  const isDemo = me?.token.name === "demo"
  const refresh = useCallback(async () => setMe(await api<Me>("/v1/me")), [])

  return (
    <AuthContext.Provider value={{ me, loading, error, signIn, startDemo, signOut, refresh, can, isDemo }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within AuthProvider")
  return ctx
}
