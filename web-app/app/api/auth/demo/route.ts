import { cookies } from "next/headers"
import { NextResponse } from "next/server"
import { ApiError, pursuitApi, TOKEN_COOKIE } from "@/lib/pursuit-api"

// "Try it without an account" / "replay tutorial": mints a fresh,
// anonymous, throwaway tenant from the Go API (token: "" so no stale
// cookie token gets sent on an otherwise-unauthenticated request) and
// stores the result exactly like a normal sign-in. 404s when the
// deployment doesn't have PURSUIT_DEMO_MODE set.
export async function POST() {
  try {
    const { token } = await pursuitApi<{ token: string }>("/v1/demo", { method: "POST", token: "" })
    const me = await pursuitApi("/v1/me", { token })
    const jar = await cookies()
    jar.set(TOKEN_COOKIE, token, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      path: "/",
      maxAge: 60 * 60 * 24 * 30,
    })
    return NextResponse.json(me)
  } catch (err) {
    const status = err instanceof ApiError ? err.status : 500
    const message = err instanceof ApiError ? err.message : "Could not start the demo."
    return NextResponse.json({ error: { message } }, { status })
  }
}
