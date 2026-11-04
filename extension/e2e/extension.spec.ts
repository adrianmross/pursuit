// End to end: the built extension in Chromium against a real `pursuit serve`.
// ATS pages are fixtures served at their real hostnames, so extraction and
// posting-URL dedupe run on real-shaped URLs. Nothing leaves the machine.

import { test, expect, type Page } from "@playwright/test";
import { OWNER, fixture, start, type Harness } from "./harness.ts";

let h: Harness;

test.describe.configure({ mode: "serial" });

function panel(page: Page) {
  return page.locator("[data-pursuit-panel]");
}

test.beforeAll(async () => {
  h = await start();
  const form = fixture("greenhouse-apply.html");
  await h.context.route("https://job-boards.greenhouse.io/**", (route) => route.fulfill({ contentType: "text/html", body: form }));
});

test.afterAll(async () => {
  await h?.stop();
});

test("pairing checks the server and token", async () => {
  const page = await h.context.newPage();
  await page.goto(`chrome-extension://${h.extensionId}/popup.html`);
  await page.getByLabel("Server").fill(h.base);
  await page.getByLabel("Token").fill("pst_wrong");
  await page.getByRole("button", { name: "Pair" }).click();
  await expect(page.getByRole("alert")).toContainText("Could not pair");

  await page.getByLabel("Token").fill(OWNER);
  await page.getByRole("button", { name: "Pair" }).click();
  await expect(page.getByText(/Paired with 127\.0\.0\.1:\d+ as /)).toBeVisible();
  await page.close();
});

test("a person saves the posting, autofills, attaches the résumé, and nothing is submitted", async () => {
  const page = await h.context.newPage();
  const url = "https://job-boards.greenhouse.io/acme/jobs/111?utm_source=linkedin";
  await page.goto(url);

  await page.getByRole("button", { name: "Open Pursuit: Autofill" }).click();
  const p = panel(page);
  await expect(p.getByRole("heading", { name: "Backend Engineer" })).toBeVisible();
  await expect(p.getByText("Acme Robotics · New York, NY · $160K–$190K")).toBeVisible();

  // Undo takes a fresh save back off the tracker.
  await p.getByRole("button", { name: "Save job" }).click();
  await p.getByRole("button", { name: "Undo" }).click();
  await expect(p.getByRole("button", { name: "Save job" })).toBeVisible();
  await expect(h.call("GET", "/v1/applications/by-url?url=" + encodeURIComponent("https://job-boards.greenhouse.io/acme/jobs/111"))).rejects.toThrow(/404/);

  await p.getByRole("button", { name: "Save job" }).click();
  await expect(p.locator("[data-slot=badge]")).toHaveText("Apply");
  await expect(p.getByRole("link", { name: /Backend Engineer.*Open in Pursuit/ })).toBeVisible();
  const app = await h.call("GET", "/v1/applications/by-url?url=" + encodeURIComponent("https://job-boards.greenhouse.io/acme/jobs/111"));
  expect(app).toMatchObject({ company: "Acme Robotics", title: "Backend Engineer", status: "to_apply", source: "greenhouse" });

  await p.getByRole("button", { name: "Autofill this form" }).click();
  // The fill is paced so a person can watch it: the die rolls, then lands.
  await expect(p.getByText("Rolling…")).toBeVisible();
  await expect(p.getByText("8 of 9")).toBeVisible({ timeout: 10_000 });
  await expect(p.getByText("Attached ada-resume.pdf")).toBeVisible();
  await expect(page.locator("#first_name")).toHaveValue("Ada");
  await expect(page.locator("#last_name")).toHaveValue("Lovelace");
  await expect(page.locator("#email")).toHaveValue("ada@example.com");
  await expect(page.locator("#phone")).toHaveValue("555-0100");
  await expect(page.locator("#linkedin")).toHaveValue("https://linkedin.com/in/ada");
  await expect(page.getByRole("radio", { name: "Yes" })).toBeChecked();
  await expect(page.locator("#sponsor-value")).toHaveValue("No");
  const file = await page.locator("#resume").evaluate((el: HTMLInputElement) => ({ name: el.files?.[0]?.name, size: el.files?.[0]?.size }));
  expect(file).toEqual({ name: "ada-resume.pdf", size: h.resume.length });
  const left = p.getByRole("region", { name: "Left for you" });
  await expect(left).toContainText("1 of 1");
  await expect(left).toContainText("Why Acme Robotics?*");
  await expect(page.locator("#why")).toHaveClass(/pursuit-unfilled/);
  await expect(page.locator("#why")).toBeFocused();
  expect(await page.evaluate(() => (window as unknown as { submissions: number }).submissions)).toBe(0);
  await p.getByRole("dialog").screenshot({ path: test.info().outputPath("panel.png") });

  const events = await h.call("GET", `/v1/applications/${app.id}/events`);
  expect(events.data.map((e: { title: string }) => e.title)).toContain("Autofilled 8 fields, 1 left");

  await page.locator("#why").fill("You build the robots I want to program.");
  await p.getByRole("button", { name: "Save my answer" }).click();
  await expect(p.getByText("Saved. Every later form reuses your answers.")).toBeVisible();
  await expect(left).toHaveCount(0);
  const profile = await h.call("GET", "/v1/profile");
  expect(profile.answers.why_acme_robotics).toBe("You build the robots I want to program.");

  await page.getByRole("button", { name: "Submit application" }).click();
  await p.getByRole("button", { name: "Mark applied" }).click();
  await expect(p.locator("[data-slot=badge]")).toHaveText("Applied");
  expect((await h.call("GET", `/v1/applications/${app.id}`)).status).toBe("applied");
  await page.close();
});

test("an agent claims approved work through the CLI and drives the same panel", async () => {
  const { application: app } = await h.call("POST", "/v1/applications/capture", {
    company: "Acme Robotics",
    title: "Backend Engineer",
    posting_url: "https://job-boards.greenhouse.io/acme/jobs/222",
  });
  const prepared = h.cli("apply", app.id, "--prepare");
  expect(["awaiting_review", "needs_input"]).toContain(prepared.application?.status ?? prepared.status);
  // Approval is the person's, on the owner token. The agent token cannot approve.
  expect(() => h.cli("approve", app.id)).toThrow();
  await h.call("POST", "/v1/approvals", { ids: [app.id] });

  const claimed = h.cli("queue", "claim", "--worker", "agent-1");
  expect(claimed.application.id).toBe(app.id);

  const page = await h.context.newPage();
  await page.goto(claimed.application.posting_url);
  await page.getByRole("button", { name: "Open Pursuit: Autofill" }).click();
  const p = panel(page);
  await expect(p.locator("[data-slot=badge]")).toHaveText("Approved");
  await p.getByRole("button", { name: "Autofill this form" }).click();
  await expect(p).toHaveAttribute("data-pursuit-state", "filled", { timeout: 10_000 });
  await expect(p).toHaveAttribute("data-pursuit-missed", "0");
  await expect(page.locator("#why")).toHaveValue("You build the robots I want to program.");
  const file = await page.locator("#resume").evaluate((el: HTMLInputElement) => el.files?.[0]?.name);
  expect(file).toBe("ada-resume.pdf");

  // The extension never submits; the agent holding the submit lease does.
  expect(await page.evaluate(() => (window as unknown as { submissions: number }).submissions)).toBe(0);
  await page.getByRole("button", { name: "Submit application" }).click();
  await expect(p.getByRole("button", { name: "Mark applied" })).toHaveCount(0);
  const done = h.cli("queue", "complete", claimed.work.id, "--worker", "agent-1", "--status", "submitted");
  expect(done.status).toBe("submitted");

  const events = await h.call("GET", `/v1/applications/${app.id}/events`);
  expect(events.data.map((e: { title: string }) => e.title)).toContain("Autofilled 9 fields");
  await page.close();
});

test("the dashboard updates in place and notifies when an agent saves a posting", async () => {
  const dash = await h.context.newPage();
  // The embedded dashboard keeps the session in a cookie; sign in the way its form does.
  expect((await dash.request.post(h.base + "/v1/auth/session", { data: { token: OWNER }, headers: { Origin: h.base } })).ok()).toBe(true);
  await dash.goto(h.base + "/applications");
  await expect(dash.getByRole("link", { name: "Backend Engineer" }).first()).toBeVisible();

  await h.call("POST", "/v1/applications/capture", { company: "Globex", title: "Data Engineer", posting_url: "https://jobs.lever.co/globex/9" }, h.agentToken);

  // No reload: the long poll picks it up.
  await expect(dash.getByRole("link", { name: "Data Engineer" })).toBeVisible({ timeout: 15_000 });
  await expect(dash.getByText("Saved Data Engineer.").first()).toBeVisible();
  await dash.getByRole("button", { name: /Notifications/ }).click();
  await dash.getByRole("tab", { name: "Everything" }).click();
  await expect(dash.getByRole("tab", { name: "Everything" })).toHaveAttribute("data-state", "active");
  await expect(dash.getByRole("dialog").getByText("Saved Data Engineer.")).toBeVisible();
  await dash.waitForTimeout(400); // let the tab transition finish so the screenshot is readable
  await dash.screenshot({ path: test.info().outputPath("dashboard.png") });
  await dash.close();
});
