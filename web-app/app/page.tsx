import { getMe } from "@/lib/pursuit-api"
import { SignOutButton } from "@/components/SignOutButton"

// Placeholder — Phase 2 replaces this with the real Home page (CTA,
// CommandBlock, import cards). This just proves the auth/BFF loop works
// end to end: middleware redirect, cookie sign-in, server-side /v1/me.
export default async function Home() {
  const me = await getMe()
  return (
    <div className="mx-auto flex max-w-xl flex-col gap-4 p-8">
      <h1 className="font-display text-xl font-semibold">Hire Pursuit</h1>
      <p className="text-muted-foreground">
        Signed in as <code className="font-mono text-foreground">{me?.token.name || "token"}</code>
        {me?.token.scopes?.length ? ` (${me.token.scopes.join(", ")})` : ""}.
      </p>
      <SignOutButton />
    </div>
  )
}
