import { EXPORT_MODE, pursuitApi } from "@/lib/pursuit-api"
import { SignInGate } from "./sign-in-gate"

export default async function SignInPage() {
  let allowTokenLogin = true
  if (!EXPORT_MODE) {
    try {
      const result = await pursuitApi<{ allow_token_login: boolean }>("/v1/auth/providers")
      allowTokenLogin = result.allow_token_login
    } catch {
      // public endpoint; leave the default if it's unreachable
    }
  }
  return <SignInGate allowTokenLogin={allowTokenLogin} />
}
