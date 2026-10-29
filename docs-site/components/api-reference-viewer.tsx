"use client"

import dynamic from "next/dynamic"

// Scalar's viewer touches the DOM at import time, so it can't be prerendered —
// load it client-only.
const ApiReferenceReact = dynamic(
  () => import("@scalar/api-reference-react").then((mod) => mod.ApiReferenceReact),
  { ssr: false },
)

// Pointed at the daemon's own OpenAPI document (see AGENTS.md: `GET /openapi.json`).
// Override for a deployed server with NEXT_PUBLIC_PURSUIT_SERVER.
const PURSUIT_SERVER = process.env.NEXT_PUBLIC_PURSUIT_SERVER || "http://127.0.0.1:8787"

export function ApiReferenceViewer() {
  return (
    <div className="api-reference-frame">
      <ApiReferenceReact
        configuration={{
          title: "pursuit API",
          url: `${PURSUIT_SERVER}/openapi.json`,
          layout: "modern",
          theme: "none",
          showSidebar: true,
          hideClientButton: false,
          withDefaultFonts: false,
          persistAuth: true,
        }}
      />
    </div>
  )
}
