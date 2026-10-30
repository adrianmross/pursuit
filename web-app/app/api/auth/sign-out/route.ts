import { cookies } from "next/headers"
import { NextResponse } from "next/server"
import { TOKEN_COOKIE } from "@/lib/pursuit-api"

export async function POST() {
  const jar = await cookies()
  jar.delete(TOKEN_COOKIE)
  return NextResponse.json({ ok: true })
}
