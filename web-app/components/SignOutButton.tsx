"use client"

import { Button } from "@/components/ui/button"
import { useAuth } from "@/lib/auth"

export function SignOutButton() {
  const { signOut } = useAuth()
  return (
    <Button variant="outline" size="sm" className="self-start" onClick={signOut}>
      Sign out
    </Button>
  )
}
