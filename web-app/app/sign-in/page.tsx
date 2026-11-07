import { EXPORT_MODE, pursuitApi } from "@/lib/pursuit-api"
import { SignInGate } from "./sign-in-gate"

export default async function SignInPage({ searchParams }: { searchParams: Promise<{ mode?: string }> }) {
  const initialMode = (await searchParams).mode === "sign-up" ? "sign-up" : "sign-in"
  let allowTokenLogin = true
  let demoAvailable = false
  if (!EXPORT_MODE) {
    try {
      const result = await pursuitApi<{ allow_token_login: boolean; demo_available: boolean }>("/v1/auth/providers")
      allowTokenLogin = result.allow_token_login
      demoAvailable = result.demo_available
    } catch {
      // public endpoint; leave the defaults if it's unreachable
    }
  }
  return <SignInGate allowTokenLogin={allowTokenLogin} demoAvailable={demoAvailable} initialMode={initialMode} />
}
