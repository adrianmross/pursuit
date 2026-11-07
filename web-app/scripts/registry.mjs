// Writes registry.json from globals.css and the component list below, then
// `shadcn build` turns it into public/r/<name>.json, served at /r/<name>.json.
// Run with `npm run registry`; commit both.
import { readFileSync, writeFileSync } from "node:fs"

const css = readFileSync(new URL("../app/globals.css", import.meta.url), "utf8")

function vars(selector) {
  const block = css.match(new RegExp(`^${selector.replace(".", "\\.")} \\{([^}]*)\\}`, "m"))
  if (!block) throw new Error("no " + selector + " block in globals.css")
  return Object.fromEntries([...block[1].matchAll(/--([\w-]+):\s*([^;]+);/g)].map((m) => [m[1], m[2].trim()]))
}

const status = { path: "lib/status.ts", type: "registry:lib" }

const registry = {
  $schema: "https://ui.shadcn.com/schema/registry.json",
  name: "hire-pursuit",
  homepage: "https://github.com/adrianmross/pursuit",
  items: [
    {
      name: "hire-pursuit",
      type: "registry:theme",
      title: "Hire Pursuit theme",
      description: "The Hire Pursuit tokens: monochrome surfaces, ink actions, brand forest, cream and gold, in light and dark.",
      dependencies: ["@fontsource-variable/outfit", "@fontsource-variable/inter", "@fontsource-variable/jetbrains-mono"],
      cssVars: {
        theme: {
          "font-heading": '"Outfit Variable", "Avenir Next", system-ui, sans-serif',
          "font-sans": '"Inter Variable", system-ui, -apple-system, "Segoe UI", sans-serif',
          "font-mono": '"JetBrains Mono Variable", ui-monospace, Menlo, monospace',
          "color-brand-forest": "var(--brand-forest)",
          "color-brand-cream": "var(--brand-cream)",
          "color-brand-gold": "var(--brand-gold)",
          "color-gold-ink": "var(--gold-ink)",
          "radius-pill": "999px",
        },
        light: vars(":root"),
        dark: vars(".dark"),
      },
    },
    {
      name: "status",
      type: "registry:lib",
      title: "Status helpers",
      description: "Status labels, tones, categories and progress-mark pieces, matching the API's status words.",
      files: [status],
    },
    {
      name: "progress-mark",
      type: "registry:component",
      title: "Progress mark",
      description: "The logo's p as a game piece: five ring pieces and a gold tail that fill as an application moves.",
      registryDependencies: [],
      files: [{ path: "components/ProgressMark.tsx", type: "registry:component" }, status],
    },
    {
      name: "status-badge",
      type: "registry:component",
      title: "Status badge",
      description: "A status chip told apart by lightness and outline; gold only for an offer.",
      registryDependencies: ["badge"],
      files: [{ path: "components/StatusBadge.tsx", type: "registry:component" }, status],
    },
    {
      name: "sketch",
      type: "registry:component",
      title: "Sketch pieces",
      description: "Hand-drawn board-game pieces (ring, dice, track, hold icons) roughened by one SVG filter, for empty and error states.",
      files: [{ path: "components/Sketch.tsx", type: "registry:component" }],
    },
  ],
}

writeFileSync(new URL("../registry.json", import.meta.url), JSON.stringify(registry, null, 2) + "\n")
