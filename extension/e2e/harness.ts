// A throwaway `pursuit serve` with a seeded profile and résumé, plus Chromium
// with the built extension loaded. The e2e spec and `npm run try` share it.

import { chromium, type BrowserContext } from "@playwright/test";
import { execFileSync, spawn } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const ROOT = resolve(import.meta.dirname, "../..");
const DIST = resolve(import.meta.dirname, "../dist");
export const OWNER = "pst_" + "e2e0".repeat(16);

export const fixture = (name: string) => readFileSync(resolve(import.meta.dirname, "../fixtures", name), "utf8");

// The smallest PDF a viewer opens: one page that says who it belongs to.
export function resumePDF(line: string): Buffer {
  const text = `BT /F1 18 Tf 72 720 Td (${line}) Tj ET`;
  const objs = [
    "<< /Type /Catalog /Pages 2 0 R >>",
    "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
    "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
    `<< /Length ${text.length} >>\nstream\n${text}\nendstream`,
    "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
  ];
  let out = "%PDF-1.4\n";
  const offsets = objs.map((o, i) => {
    const at = out.length;
    out += `${i + 1} 0 obj\n${o}\nendobj\n`;
    return at;
  });
  const xref = out.length;
  out += `xref\n0 ${objs.length + 1}\n0000000000 65535 f \n` + offsets.map((o) => String(o).padStart(10, "0") + " 00000 n \n").join("");
  out += `trailer\n<< /Size ${objs.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`;
  return Buffer.from(out, "latin1");
}

export const PROFILE = {
  first_name: "Ada",
  last_name: "Lovelace",
  email: "ada@example.com",
  phone: "555-0100",
  linkedin: "https://linkedin.com/in/ada",
  work_authorization: "Yes",
  visa_sponsorship: "No",
  cover_letter: "I like turning careful analysis into machines that work.",
};

async function freePort(): Promise<number> {
  return new Promise((done) => {
    const s = createServer().listen(0, "127.0.0.1", () => {
      const port = (s.address() as { port: number }).port;
      s.close(() => done(port));
    });
  });
}

export type Harness = Awaited<ReturnType<typeof start>>;

export async function start(opts: { headless?: boolean; port?: number } = {}) {
  const tmp = mkdtempSync(join(tmpdir(), "pursuit-e2e-"));
  writeFileSync(join(tmp, "config.json"), "{}");
  // Keep the developer's own pursuit config and PURSUIT_* variables out of the run.
  const env: Record<string, string> = { PURSUIT_CONFIG: join(tmp, "config.json") };
  for (const [k, v] of Object.entries(process.env)) if (v !== undefined && !k.startsWith("PURSUIT_")) env[k] = v;

  const bin = process.env.PURSUIT_BIN || join(tmp, "pursuit");
  if (!process.env.PURSUIT_BIN) execFileSync("go", ["build", "-o", bin, "./cmd/pursuit"], { cwd: ROOT, stdio: "inherit" });
  const base = `http://127.0.0.1:${opts.port || (await freePort())}`;
  const server = spawn(bin, ["serve", "--addr", base.slice(7), "--database-url", `sqlite://${join(tmp, "pursuit.db")}`, "--bootstrap-token", OWNER, "--cors-origins", "chrome-extension://*", "--log-level", "warn"], {
    env,
    stdio: ["ignore", "inherit", "inherit"],
  });
  for (let i = 0; i < 100; i++) {
    if (await fetch(base + "/readyz").then((r) => r.ok, () => false)) break;
    await new Promise((r) => setTimeout(r, 100));
  }

  async function call(method: string, path: string, body?: unknown, token = OWNER) {
    const res = await fetch(base + path, {
      method,
      headers: { Authorization: "Bearer " + token, ...(body ? { "Content-Type": "application/json" } : {}) },
      body: body ? JSON.stringify(body) : undefined,
    });
    const data = await res.json();
    if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${JSON.stringify(data)}`);
    return data;
  }

  const agentToken: string = (await call("POST", "/v1/tokens", { name: "agent-1", scopes: ["read", "write"] })).secret;
  for (const [key, answer] of Object.entries(PROFILE)) await call("POST", "/v1/standing", { key, answer });
  const resume = resumePDF("Ada Lovelace - Analyst, Engineer");
  const upload = new FormData();
  upload.set("kind", "base_resume");
  upload.set("file", new Blob([resume], { type: "application/pdf" }), "ada-resume.pdf");
  const added = await fetch(base + "/v1/documents", { method: "POST", headers: { Authorization: "Bearer " + OWNER }, body: upload });
  if (added.status !== 201) throw new Error("résumé upload: " + added.status);

  // The agent's side goes through the real CLI, the way AGENTS.md tells agents to work.
  function cli(...args: string[]) {
    const out = execFileSync(bin, args, { env: { ...env, PURSUIT_SERVER: base, PURSUIT_TOKEN: agentToken }, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
    return JSON.parse(out);
  }

  const context: BrowserContext = await chromium.launchPersistentContext("", {
    channel: "chromium",
    headless: opts.headless ?? true,
    viewport: opts.headless === false ? null : undefined,
    args: [`--disable-extensions-except=${DIST}`, `--load-extension=${DIST}`],
  });
  const worker = context.serviceWorkers()[0] || (await context.waitForEvent("serviceworker"));
  const extensionId = new URL(worker.url()).host;

  // Pair without the popup, as if the person had already done it.
  async function pair(token = OWNER) {
    await worker.evaluate(([server, t]) => chrome.storage.local.set({ server, token: t }), [base, token]);
  }

  async function stop() {
    await context.close().catch(() => undefined);
    server.kill();
    rmSync(tmp, { recursive: true, force: true });
  }

  return { base, bin, tmp, resume, agentToken, context, extensionId, call, cli, pair, stop };
}
