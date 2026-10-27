import { createContext, useCallback, useContext, useState, type ReactNode } from "react"
import { api, ApiError, getToken, setToken } from "@/lib/api"
import type { Me } from "@/lib/types"

type AuthState = {
  me: Me | null
  loading: boolean
  error: string | null
  signIn: (token: string) => Promise<void>
  signOut: () => void
  can: (scope: string) => boolean
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const result = await api<Me>("/v1/me")
      setMe(result)
      setError(null)
    } catch (err) {
      setMe(null)
      if (err instanceof ApiError && getToken() === "") {
        setError(null)
      } else {
        setError(err instanceof Error ? err.message : "Sign-in failed")
      }
    } finally {
      setLoading(false)
    }
  }, [])

  const signIn = useCallback(
    async (token: string) => {
      setToken(token)
      await load()
    },
    [load],
  )

  const signOut = useCallback(() => {
    setToken("")
    setMe(null)
  }, [])

  const can = useCallback(
    (scope: string) => {
      const scopes = me?.token.scopes || []
      return scopes.includes(scope) || (scope === "read" && scopes.includes("write"))
    },
    [me],
  )

  useState(() => {
    void load()
  })

  return <AuthContext.Provider value={{ me, loading, error, signIn, signOut, can }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within AuthProvider")
  return ctx
}
