import type { NextConfig } from "next"

// Self-hosted (NEXT_PUBLIC_EXPORT_MODE=1) builds to a static `out/` dir,
// embedded into the Go binary (internal/dashboard, go:embed dist) exactly
// like the old Vite build was — no Node process in that deployment mode.
const exportMode = process.env.NEXT_PUBLIC_EXPORT_MODE === "1"

const nextConfig: NextConfig = {
  // appkit-ui ships TypeScript source.
  transpilePackages: ["@adrianmross/appkit-ui"],
  ...(exportMode ? { output: "export" as const } : {}),
  // pursuit-preview runs the docs dev server beside this one; the real site and
  // the embedded binary serve both from one origin, so /docs goes there too.
  ...(!exportMode && process.env.PURSUIT_DOCS_URL
    ? { rewrites: async () => [{ source: "/docs/:path*", destination: `${process.env.PURSUIT_DOCS_URL}/docs/:path*` }] }
    : {}),
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
}

export default nextConfig
