import { cookies } from "next/headers"
import { NextResponse, type NextRequest } from "next/server"
import { API_BASE, TOKEN_COOKIE } from "@/lib/pursuit-api"

// Every client component talks to the Go API through here. Reads the
// httpOnly cookie server-side, attaches it as a bearer header, forwards the
// request verbatim, and streams the response back — the browser never sees
// the token.
async function forward(request: NextRequest, path: string[]) {
  const token = (await cookies()).get(TOKEN_COOKIE)?.value
  const headers: Record<string, string> = {}
  if (token) headers.Authorization = "Bearer " + token
  const contentType = request.headers.get("content-type")
  if (contentType) headers["Content-Type"] = contentType

  const url = API_BASE + "/" + path.join("/") + request.nextUrl.search
  const hasBody = request.method !== "GET" && request.method !== "HEAD"

  const response = await fetch(url, {
    method: request.method,
    headers,
    body: hasBody ? await request.arrayBuffer() : undefined,
    cache: "no-store",
  })

  const body = await response.arrayBuffer()
  return new NextResponse(body, {
    status: response.status,
    headers: {
      "Content-Type": response.headers.get("content-type") || "application/json",
    },
  })
}

export async function GET(request: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  return forward(request, (await params).path)
}
export async function POST(request: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  return forward(request, (await params).path)
}
export async function PATCH(request: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  return forward(request, (await params).path)
}
export async function DELETE(request: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  return forward(request, (await params).path)
}
