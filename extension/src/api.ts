// The Pursuit HTTP API, shared by the background worker, the popup and the panel.

export type Pairing = { server: string; token: string };
export type ApiRequest = { method: string; path: string; body?: unknown };

export type App = {
  id: string;
  status: string;
  outcome?: string;
  stage?: number;
  version: number;
  title: string;
  company: string;
  posting_url?: string;
};

export async function api(p: Pairing, method: string, path: string, body?: unknown): Promise<unknown> {
  if (!p.server || !p.token) throw Object.assign(new Error("Pair this browser with your Pursuit server first."), { code: "unpaired" });
  if (!path.startsWith("/v1/")) throw new Error("not an API path");
  const res = await fetch(p.server.replace(/\/$/, "") + path, {
    method,
    headers: {
      Authorization: "Bearer " + p.token,
      ...(body ? { "Content-Type": "application/json" } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let data: { error?: { message?: string; code?: string } } | null = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    // A proxy or sign-in page answered instead of the API.
  }
  if (!res.ok) {
    throw Object.assign(new Error(data?.error?.message || res.statusText || "Request failed"), {
      code: data?.error?.code || String(res.status),
    });
  }
  return data;
}

// resume_pdf artifacts are always base64; a document is base64 unless its type is text.
export function decodeFile(body: string, filename: string, contentType: string, base64: boolean): File {
  const bytes = base64 ? Uint8Array.from(atob(body), (c) => c.charCodeAt(0)) : new TextEncoder().encode(body);
  return new File([bytes], filename || "resume.pdf", { type: contentType || "application/pdf" });
}
