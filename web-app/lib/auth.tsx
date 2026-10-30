"use client"

import { createContext, useCallback, useContext, useState, type ReactNode } from "react"
import { api, ApiError, EXPORT_MODE, setToken } from "@/lib/api-client"
import type { Me } from "@/lib/types"

type AuthState = {
  me: Me | null
  error: string | null
  signIn: (token: string) => Promise<void>
  signOut: () => Promise<void>
  can: (scope: string) => boolean
}

const AuthContext = createContext<AuthState | null>(null)

// On the hosted build, `me` is fetched server-side (middleware already
// redirects anonymous requests to /sign-in, and the root layout passes the
// server-fetched identity down) — this provider just holds it in context.
// In export mode there's no server fetch to seed it from, so it starts null
// and SignIn populates it directly after a successful token check.
export function AuthProvider({ initialMe, children }: { initialMe: Me | null; children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(initialMe)
  const [error, setError] = useState<string | null>(null)

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

  return <AuthContext.Provider value={{ me, error, signIn, signOut, can }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within AuthProvider")
  return ctx
}
