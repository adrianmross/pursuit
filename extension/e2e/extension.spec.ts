// End to end: the built extension in Chromium against a real `pursuit serve`.
// ATS pages are fixtures served at their real hostnames, so extraction and
// posting-URL dedupe run on real-shaped URLs. Nothing leaves the machine.

import { test, expect, chromium, type BrowserContext, type Page } from "@playwright/test";
import { execFileSync, spawn, type ChildProcess } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const ROOT = resolve(import.meta.dirname, "../..");
const DIST = resolve(import.meta.dirname, "../dist");
const FORM = readFileSync(resolve(import.meta.dirname, "../fixtures/greenhouse-apply.html"), "utf8");
const OWNER = "pst_" + "e2e0".repeat(16);
const RESUME = Buffer.from("%PDF-1.4\n% Ada Lovelace résumé\n%%EOF\n");

let tmp: string;
let bin: string;
let server: ChildProcess;
let base: string;
let context: BrowserContext;
let extensionId: string;
let agentToken: string;

test.describe.configure({ mode: "serial" });

async function freePort(): Promise<number> {
  return new Promise((done) => {
    const s = createServer().listen(0, "127.0.0.1", () => {
      const port = (s.address() as { port: number }).port;
      s.close(() => done(port));
    });
  });
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

// The agent's side goes through the real CLI, the way AGENTS.md tells agents to work.
function cli(...args: string[]) {
  const out = execFileSync(bin, args, { env: { ...env(), PURSUIT_SERVER: base, PURSUIT_TOKEN: agentToken }, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
  return JSON.parse(out);
}

// Keep the developer's own pursuit config and PURSUIT_* variables out of the run.
function env() {
  const out: Record<string, string> = { PURSUIT_CONFIG: join(tmp, "config.json") };
  for (const [k, v] of Object.entries(process.env)) if (v !== undefined && !k.startsWith("PURSUIT_")) out[k] = v;
  return out;
}

function panel(page: Page) {
  return page.locator("[data-pursuit-panel]");
}

test.beforeAll(async () => {
  tmp = mkdtempSync(join(tmpdir(), "pursuit-e2e-"));
  writeFileSync(join(tmp, "config.json"), "{}");
  bin = process.env.PURSUIT_BIN || join(tmp, "pursuit");
  if (!process.env.PURSUIT_BIN) execFileSync("go", ["build", "-o", bin, "./cmd/pursuit"], { cwd: ROOT, stdio: "inherit" });
  base = `http://127.0.0.1:${await freePort()}`;
  server = spawn(bin, ["serve", "--addr", base.slice(7), "--database-url", `sqlite://${join(tmp, "e2e.db")}`, "--bootstrap-token", OWNER, "--cors-origins", "chrome-extension://*", "--log-level", "warn"], {
    env: env(),
    stdio: ["ignore", "inherit", "inherit"],
  });
  for (let i = 0; i < 100; i++) {
    const ok = await fetch(base + "/readyz").then((r) => r.ok, () => false);
    if (ok) break;
    await new Promise((r) => setTimeout(r, 100));
  }

  agentToken = (await call("POST", "/v1/tokens", { name: "agent-1", scopes: ["read", "write"] })).secret;
  for (const [key, answer] of Object.entries({
    first_name: "Ada",
    last_name: "Lovelace",
    email: "ada@example.com",
    phone: "555-0100",
    linkedin: "https://linkedin.com/in/ada",
    work_authorization: "Yes",
    visa_sponsorship: "No",
  })) {
    await call("POST", "/v1/standing", { key, answer });
  }
  const upload = new FormData();
  upload.set("kind", "base_resume");
  upload.set("file", new Blob([RESUME], { type: "application/pdf" }), "ada-resume.pdf");
  const added = await fetch(base + "/v1/documents", { method: "POST", headers: { Authorization: "Bearer " + OWNER }, body: upload });
  expect(added.status).toBe(201);

  context = await chromium.launchPersistentContext("", {
    channel: "chromium",
    args: [`--disable-extensions-except=${DIST}`, `--load-extension=${DIST}`],
  });
  const worker = context.serviceWorkers()[0] || (await context.waitForEvent("serviceworker"));
  extensionId = new URL(worker.url()).host;
  await context.route("https://job-boards.greenhouse.io/**", (route) => route.fulfill({ contentType: "text/html", body: FORM }));
});

test.afterAll(async () => {
  await context?.close();
  server?.kill();
  rmSync(tmp, { recursive: true, force: true });
});

test("pairing checks the server and token", async () => {
  const page = await context.newPage();
  await page.goto(`chrome-extension://${extensionId}/popup.html`);
  await page.getByLabel("Server").fill(base);
  await page.getByLabel("Token").fill("pst_wrong");
  await page.getByRole("button", { name: "Pair" }).click();
  await expect(page.getByRole("alert")).toContainText("Could not pair");

  await page.getByLabel("Token").fill(OWNER);
  await page.getByRole("button", { name: "Pair" }).click();
  await expect(page.getByText(/Paired with 127\.0\.0\.1:\d+ as /)).toBeVisible();
  await page.close();
});

test("a person saves the posting, autofills, attaches the résumé, and nothing is submitted", async () => {
  const page = await context.newPage();
  const url = "https://job-boards.greenhouse.io/acme/jobs/111?utm_source=linkedin";
  await page.goto(url);

  await page.getByRole("button", { name: "Open Pursuit: Autofill" }).click();
  const p = panel(page);
  await expect(p.getByRole("heading", { name: "Backend Engineer" })).toBeVisible();
  await expect(p.getByText("Acme Robotics · New York, NY · $160K–$190K")).toBeVisible();
  await expect(p.getByText("Not tracked yet.")).toBeVisible();

  await p.getByRole("button", { name: "Save job" }).click();
  await expect(p.getByText("Saved as To apply.")).toBeVisible();
  await expect(p.locator(".chip")).toHaveText("To apply");
  const app = await call("GET", "/v1/applications/by-url?url=" + encodeURIComponent("https://job-boards.greenhouse.io/acme/jobs/111"));
  expect(app).toMatchObject({ company: "Acme Robotics", title: "Backend Engineer", status: "to_apply", source: "greenhouse" });

  await p.getByRole("button", { name: "Autofill application" }).click();
  await expect(p.getByText(/Filled \d+ fields\. Attached ada-resume\.pdf\./)).toBeVisible();
  await expect(page.locator("#first_name")).toHaveValue("Ada");
  await expect(page.locator("#last_name")).toHaveValue("Lovelace");
  await expect(page.locator("#email")).toHaveValue("ada@example.com");
  await expect(page.locator("#phone")).toHaveValue("555-0100");
  await expect(page.locator("#linkedin")).toHaveValue("https://linkedin.com/in/ada");
  await expect(page.getByRole("radio", { name: "Yes" })).toBeChecked();
  await expect(page.locator("#sponsor-value")).toHaveValue("No");
  const file = await page.locator("#resume").evaluate((el: HTMLInputElement) => ({ name: el.files?.[0]?.name, size: el.files?.[0]?.size }));
  expect(file).toEqual({ name: "ada-resume.pdf", size: RESUME.length });
  await expect(p.getByRole("listitem")).toHaveText(["Why Acme Robotics?*"]);
  await expect(page.locator("#why")).toHaveClass(/pursuit-unfilled/);
  expect(await page.evaluate(() => (window as unknown as { submissions: number }).submissions)).toBe(0);
  await p.screenshot({ path: test.info().outputPath("panel.png") });

  const events = await call("GET", `/v1/applications/${app.id}/events`);
  expect(events.data.map((e: { title: string }) => e.title)).toContain("Autofilled 8 fields, 1 left");

  await page.locator("#why").fill("You build the robots I want to program.");
  await p.getByRole("button", { name: "Save my answers" }).click();
  await expect(p.getByText("Saved 1 answer to your profile.", { exact: false })).toBeVisible();
  const profile = await call("GET", "/v1/profile");
  expect(profile.answers.why_acme_robotics).toBe("You build the robots I want to program.");

  await page.getByRole("button", { name: "Submit application" }).click();
  await p.getByRole("button", { name: "Mark applied" }).click();
  await expect(p.locator(".chip")).toHaveText("Applied");
  expect((await call("GET", `/v1/applications/${app.id}`)).status).toBe("applied");
  await page.close();
});

test("an agent claims approved work through the CLI and drives the same panel", async () => {
  const { application: app } = await call("POST", "/v1/applications/capture", {
    company: "Acme Robotics",
    title: "Backend Engineer",
    posting_url: "https://job-boards.greenhouse.io/acme/jobs/222",
  });
  const prepared = cli("apply", app.id, "--prepare");
  expect(["awaiting_review", "needs_input"]).toContain(prepared.application?.status ?? prepared.status);
  // Approval is the person's, on the owner token. The agent token cannot approve.
  expect(() => cli("approve", app.id)).toThrow();
  await call("POST", "/v1/approvals", { ids: [app.id] });

  const claimed = cli("queue", "claim", "--worker", "agent-1");
  expect(claimed.application.id).toBe(app.id);

  const page = await context.newPage();
  await page.goto(claimed.application.posting_url);
  await page.getByRole("button", { name: "Open Pursuit: Autofill" }).click();
  const p = panel(page);
  await expect(p.locator(".chip")).toHaveText("Approved");
  await p.getByRole("button", { name: "Autofill application" }).click();
  await expect(p).toHaveAttribute("data-pursuit-state", "filled");
  await expect(p).toHaveAttribute("data-pursuit-missed", "0");
  await expect(page.locator("#why")).toHaveValue("You build the robots I want to program.");
  const file = await page.locator("#resume").evaluate((el: HTMLInputElement) => el.files?.[0]?.name);
  expect(file).toBe("ada-resume.pdf");

  // The extension never submits; the agent holding the submit lease does.
  expect(await page.evaluate(() => (window as unknown as { submissions: number }).submissions)).toBe(0);
  await page.getByRole("button", { name: "Submit application" }).click();
  await expect(p.getByRole("button", { name: "Mark applied" })).toHaveCount(0);
  const done = cli("queue", "complete", claimed.work.id, "--worker", "agent-1", "--status", "submitted");
  expect(done.status).toBe("submitted");

  const events = await call("GET", `/v1/applications/${app.id}/events`);
  expect(events.data.map((e: { title: string }) => e.title)).toContain("Autofilled 9 fields");
  await page.close();
});
