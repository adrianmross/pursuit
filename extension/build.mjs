import { build } from "esbuild";
import { execFileSync } from "node:child_process";
import { cp, mkdir, readFile } from "node:fs/promises";

await mkdir("dist/fonts", { recursive: true });

// Tailwind compiles the panel's stylesheet first; the panel puts it in its shadow root.
execFileSync("npx", ["@tailwindcss/cli", "-i", "src/panel.css", "-o", "dist/.panel.css", "--minify"], { stdio: "inherit" });
const panelCSS = await readFile("dist/.panel.css", "utf8");

const common = {
  bundle: true,
  outdir: "dist",
  target: "chrome109",
  loader: { ".css": "text", ".svg": "text" },
  define: { __PANEL_CSS__: JSON.stringify(panelCSS), "process.env.NODE_ENV": '"production"' },
  minify: true,
  logLevel: "info",
};
// content.js runs in every frame, so it stays a small classic script; the
// React panel is an ES module it imports only on postings and forms.
await build({ ...common, entryPoints: ["src/background.ts", "src/content.ts", "src/popup.ts"], format: "iife" });
await build({ ...common, entryPoints: ["src/panel.tsx"], format: "esm" });

await cp("src/popup.html", "dist/popup.html");
await cp("src/popup.css", "dist/popup.css");
await cp("src/tokens.css", "dist/tokens.css");
await cp("manifest.json", "dist/manifest.json");
await cp("../web-app/public/brand/hire-pursuit-mark.svg", "dist/mark.svg");
await cp("../web-app/public/apple-touch-icon.png", "dist/icon.png");
await cp("node_modules/@fontsource-variable/inter/files/inter-latin-wght-normal.woff2", "dist/fonts/inter.woff2");
await cp("node_modules/@fontsource-variable/outfit/files/outfit-latin-wght-normal.woff2", "dist/fonts/outfit.woff2");
