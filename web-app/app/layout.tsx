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
      </body>
    </html>
  )
}
