import { cookies } from "next/headers"
import { NextResponse } from "next/server"
import { ApiError, pursuitApi, TOKEN_COOKIE } from "@/lib/pursuit-api"

// Accepts either a Pursuit-issued token (pst_...) or a Better Auth JWT —
// the Go API's authenticate() already dispatches on shape, so this just
// verifies whatever it's given against GET /v1/me, then stores it in an
// httpOnly cookie. The browser never holds the raw token.
export async function POST(request: Request) {
  const { token } = (await request.json()) as { token?: string }
  if (!token) {
    return NextResponse.json({ error: { message: "Missing token" } }, { status: 400 })
  }

  try {
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
    const status = err instanceof ApiError ? err.status : 401
    const message = err instanceof ApiError ? err.message : "That token was rejected."
    return NextResponse.json({ error: { message } }, { status })
  }
}
