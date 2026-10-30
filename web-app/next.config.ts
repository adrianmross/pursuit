import type { NextConfig } from "next"

// Self-hosted (NEXT_PUBLIC_EXPORT_MODE=1) builds to a static `out/` dir,
// embedded into the Go binary (internal/dashboard, go:embed dist) exactly
// like the old Vite build was — no Node process in that deployment mode.
const exportMode = process.env.NEXT_PUBLIC_EXPORT_MODE === "1"

const nextConfig: NextConfig = {
  ...(exportMode ? { output: "export" as const } : {}),
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
