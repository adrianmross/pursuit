import { build } from "esbuild";
import { cp, mkdir } from "node:fs/promises";

await mkdir("dist/fonts", { recursive: true });
await build({
  entryPoints: ["src/background.ts", "src/content.ts", "src/popup.ts"],
  bundle: true,
  outdir: "dist",
  format: "iife",
  target: "chrome109",
  loader: { ".css": "text" },
  logLevel: "info",
});
await cp("src/popup.html", "dist/popup.html");
await cp("src/popup.css", "dist/popup.css");
await cp("src/tokens.css", "dist/tokens.css");
await cp("manifest.json", "dist/manifest.json");
await cp("../web-app/public/brand/hire-pursuit-mark.svg", "dist/mark.svg");
await cp("../web-app/public/apple-touch-icon.png", "dist/icon.png");
await cp("node_modules/@fontsource-variable/inter/files/inter-latin-wght-normal.woff2", "dist/fonts/inter.woff2");
await cp("node_modules/@fontsource-variable/outfit/files/outfit-latin-wght-normal.woff2", "dist/fonts/outfit.woff2");
