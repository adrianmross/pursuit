"use client"

import posthog from "posthog-js"
import { useEffect } from "react"

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  useEffect(() => {
    posthog.captureException(error)
  }, [error])

  return (
    <html lang="en">
      <body className="min-h-svh">
        <main>
          <h1>Something went wrong</h1>
          <p>Try again or return to the home page.</p>
          <button onClick={reset}>Try again</button>
        </main>
      </body>
    </html>
  )
}
