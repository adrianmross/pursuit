import nextra from "nextra"

const withNextra = nextra({
  defaultShowCopyCode: true,
})

/** @type {import('next').NextConfig} */
const nextConfig = {
  poweredByHeader: false,
  // Served at /docs by the daemon, same origin as the dashboard and API.
  basePath: "/docs",
  // Static export; the Go server embeds and serves it (see Makefile `web`).
  output: "export",
  // The dev-mode indicator sits bottom-left, on top of Nextra's own
  // theme-switch/collapse-sidebar buttons there — turn it off.
  devIndicators: false,
}

export default withNextra(nextConfig)
