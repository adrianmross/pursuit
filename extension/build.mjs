import { build } from "esbuild";
import { cp, mkdir } from "node:fs/promises";

await mkdir("dist", { recursive: true });
await build({
  entryPoints: ["src/content.ts", "src/popup.ts"],
  bundle: true,
  outdir: "dist",
  format: "iife",
  target: "chrome109",
  logLevel: "info",
});
await cp("src/popup.html", "dist/popup.html");
await cp("src/popup.css", "dist/popup.css");
await cp("manifest.json", "dist/manifest.json");
