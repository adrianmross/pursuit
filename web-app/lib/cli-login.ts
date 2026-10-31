import { EXPORT_MODE, pursuitApi } from "@/lib/pursuit-api"

// Server-side, hosted build only: whether the inline sign-in on the CLI
// consent pages offers the paste-a-token form. Export mode has no server
// here; SignInForm checks /v1/auth/providers client-side instead.
export async function allowTokenLogin(): Promise<boolean> {
  if (EXPORT_MODE) return true
  try {
    return (await pursuitApi<{ allow_token_login: boolean }>("/v1/auth/providers")).allow_token_login
  } catch {
    return true
  }
}
