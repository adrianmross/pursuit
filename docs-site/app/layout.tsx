import type { Metadata } from "next"
import { Footer, Layout, Navbar } from "nextra-theme-docs"
import { Banner } from "nextra/components"
import { getPageMap } from "nextra/page-map"
import "nextra-theme-docs/style.css"
import "./globals.css"

export const metadata: Metadata = {
  title: {
    default: "Hire Pursuit docs",
    template: "%s — Hire Pursuit docs",
  },
  // Same brand icon as the dashboard; served by the daemon at the root.
  icons: {
    icon: [{ url: "/favicon.ico", sizes: "48x48" }, { url: "/favicon.svg", type: "image/svg+xml" }],
    apple: "/apple-touch-icon.png",
  },
  description: "API and CLI reference for pursuit, a self-hosted job-application tracker.",
}

const navbar = (
  <Navbar logo={
      <>
        <img className="brand-logo brand-logo-light" src="/docs/brand/hire-pursuit-wordmark.svg" alt="Hire Pursuit" />
        <img className="brand-logo brand-logo-dark" src="/docs/brand/hire-pursuit-wordmark-reverse.svg" alt="Hire Pursuit" />
        <span style={{ marginLeft: 10, opacity: 0.6, fontSize: 13 }}>docs</span>
      </>
    } projectLink="https://github.com/adrianmross/pursuit">
    <a href="/reference" className="x:text-sm x:font-medium">
      API reference
    </a>
    <a href="/" className="x:text-sm x:font-medium">
      Dashboard
    </a>
  </Navbar>
)

// Same footer as the dashboard (web-app/components/SiteFooter.tsx); keep in step.
const footer = (
  <Footer className="hp-footer">
    <span className="hp-footer-brand">
      <img className="brand-logo-light" src="/docs/brand/hire-pursuit-mark.svg" alt="" />
      <img className="brand-logo-dark" src="/docs/brand/hire-pursuit-mark-reverse.svg" alt="" />
      Hire Pursuit
    </span>
    <p className="hp-footer-credit">
      <span>
        Built with
        <svg aria-label="love" role="img" viewBox="0 0 24 24" className="hp-heart">
          <path d="M2 9.5a5.5 5.5 0 0 1 9.591-3.676.56.56 0 0 0 .818 0A5.49 5.49 0 0 1 22 9.5c0 2.29-1.5 4-3 5.5l-5.492 5.313a2 2 0 0 1-3 .019L5 15c-1.5-1.5-3-3.2-3-5.5" />
        </svg>
        by Adrian Ross
      </span>
      <span aria-hidden className="hp-footer-dot">·</span>
      <a href="https://github.com/adrianmross/pursuit/blob/main/LICENSE">MIT License © 2026</a>
    </p>
    <nav className="hp-footer-nav">
      <a href="/docs/">Docs</a>
      <a href="/reference">Reference</a>
      <a href="https://github.com/adrianmross/pursuit">GitHub</a>
    </nav>
  </Footer>
)

const banner = (
  <Banner storageKey="pursuit-docs-pre">
    Pre-release — pursuit has no versioned releases yet. Everything here can change.
  </Banner>
)

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const pageMap = await getPageMap()
  return (
    <html lang="en" dir="ltr" suppressHydrationWarning>
      <body>
        <Layout
          banner={banner}
          navbar={navbar}
          footer={footer}
          pageMap={pageMap}
          docsRepositoryBase="https://github.com/adrianmross/pursuit/tree/main/docs-site"
        >
          {children}
        </Layout>
      </body>
    </html>
  )
}
