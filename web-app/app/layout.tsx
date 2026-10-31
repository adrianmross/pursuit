import type { Metadata } from "next"
import "./globals.css"
import { getMe } from "@/lib/pursuit-api"
import { Providers } from "./providers"

export const metadata: Metadata = {
  title: {
    default: "Hire Pursuit",
    template: "%s — Hire Pursuit",
  },
  description: "A self-hosted job-application tracker.",
  icons: {
    icon: [{ url: "/favicon.ico", sizes: "48x48" }, { url: "/favicon.svg", type: "image/svg+xml" }],
    apple: "/apple-touch-icon.png",
  },
}

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const me = await getMe()
  return (
    <html lang="en" suppressHydrationWarning>
      <body className="min-h-svh">
        <Providers initialMe={me}>{children}</Providers>
        {/* Cloudflare Web Analytics (page views + Core Web Vitals). Read at build time; unset in self-hosted builds. */}
        {process.env.PURSUIT_CF_BEACON_TOKEN && (
          <script
            defer
            src="https://static.cloudflareinsights.com/beacon.min.js"
            data-cf-beacon={JSON.stringify({ token: process.env.PURSUIT_CF_BEACON_TOKEN })}
          />
        )}
      </body>
    </html>
  )
}
