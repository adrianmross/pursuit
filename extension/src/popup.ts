import { normalizeKey } from "./autofill";

type Job = { company: string; title: string; location: string; salary: string; url: string };
type App = {
  id: string;
  status: string;
  version: number;
  title: string;
  company: string;
};
type FillReport = { filled: string[]; missed: string[] };

const STATUSES = [
  "to_apply",
  "preparing",
  "awaiting_review",
  "needs_input",
  "applied",
  "interviewing",
  "offer",
  "rejected",
  "withdrawn",
];

const $ = (id: string) => document.getElementById(id) as HTMLElement;

function storageGet(): Promise<{ server?: string; token?: string }> {
  return new Promise((resolve) => chrome.storage.local.get(["server", "token"], resolve));
}

function storageSet(values: { server: string; token: string }): Promise<void> {
  return new Promise((resolve) => chrome.storage.local.set(values, () => resolve()));
}

async function api(server: string, token: string, method: string, path: string, body?: unknown): Promise<unknown> {
  const res = await fetch(server.replace(/\/$/, "") + path, {
    method,
    headers: {
      Authorization: "Bearer " + token,
      ...(body ? { "Content-Type": "application/json" } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) {
    const message = data?.error?.message || res.statusText;
    throw new Error(message);
  }
  return data;
}

function setError(message: string) {
  $("error").textContent = message;
}

function fillStatus(select: HTMLSelectElement, current: string) {
  const values = STATUSES.includes(current) || !current ? STATUSES : [current, ...STATUSES];
  select.replaceChildren();
  for (const value of values) {
    const opt = document.createElement("option");
    opt.value = value;
    opt.textContent = value.replace(/_/g, " ");
    select.appendChild(opt);
  }
  if (current) select.value = current;
}

async function activeTab(): Promise<chrome.tabs.Tab | undefined> {
  const tabs = await new Promise<chrome.tabs.Tab[]>((resolve) =>
    chrome.tabs.query({ active: true, currentWindow: true }, resolve),
  );
  return tabs[0];
}

function send(tabId: number, message: unknown): Promise<unknown> {
  return new Promise((resolve, reject) => {
    chrome.tabs.sendMessage(tabId, message, (response) => {
      const err = chrome.runtime.lastError;
      if (err) reject(new Error(err.message));
      else resolve(response);
    });
  });
}

let current: { server: string; token: string; job?: Job; app?: App } = { server: "", token: "" };

async function refresh() {
  setError("");
  const pair = $("pair");
  const jobEl = $("job");
  const saved = await storageGet();
  current.server = saved.server || "";
  current.token = saved.token || "";
  if (!current.server || !current.token) {
    pair.hidden = false;
    jobEl.hidden = true;
    ($("server") as HTMLInputElement).value = current.server;
    return;
  }
  pair.hidden = true;
  jobEl.hidden = false;
  const tab = await activeTab();
  if (!tab?.id || !tab.url || !/^https?:/.test(tab.url)) {
    $("page").textContent = "This tab is not a web page.";
    ($("save-job") as HTMLButtonElement).disabled = true;
    return;
  }
  let extracted: { job?: Job };
  try {
    extracted = (await send(tab.id, { type: "extract" })) as { job?: Job };
  } catch {
    $("page").textContent = "Pursuit cannot read this page. Reload it and open the popup again.";
    return;
  }
  current.job = extracted.job;
  const job = current.job;
  if (!job?.title) {
    $("page").textContent = "No job posting detected on this page.";
    ($("save-job") as HTMLButtonElement).disabled = true;
    ($("status") as HTMLSelectElement).disabled = true;
    return;
  }
  $("page").textContent = `${job.title}${job.company ? " · " + job.company : ""}`;
  let app: App | undefined;
  try {
    app = (await api(
      current.server,
      current.token,
      "GET",
      "/v1/applications/by-url?url=" + encodeURIComponent(job.url),
    )) as App;
  } catch (err) {
    if (!/not found/i.test(String(err))) setError(String(err instanceof Error ? err.message : err));
  }
  current.app = app;
  const select = $("status") as HTMLSelectElement;
  if (app) {
    $("status-line").textContent = "Tracked";
    fillStatus(select, app.status);
    select.disabled = false;
    ($("save-job") as HTMLButtonElement).disabled = true;
    ($("save-job") as HTMLButtonElement).textContent = "Saved";
  } else {
    $("status-line").textContent = "Not tracked";
    fillStatus(select, "to_apply");
    select.disabled = true;
    ($("save-job") as HTMLButtonElement).disabled = false;
    ($("save-job") as HTMLButtonElement).textContent = "Save job";
  }
  await showResume(app?.id);
}

async function showResume(id?: string) {
  const button = $("resume") as HTMLButtonElement;
  button.hidden = true;
  if (!id) return;
  try {
    const listed = (await api(current.server, current.token, "GET", `/v1/applications/${id}/artifacts`)) as {
      data: { id: string; kind: string; filename: string }[];
    };
    const resume = listed.data?.find((a) => a.kind === "resume_pdf");
    if (!resume) return;
    button.hidden = false;
    button.onclick = async () => {
      const art = (await api(current.server, current.token, "GET", `/v1/artifacts/${resume.id}`)) as { body: string; filename: string };
      const bytes = Uint8Array.from(atob(art.body), (c) => c.charCodeAt(0));
      const url = URL.createObjectURL(new Blob([bytes], { type: "application/pdf" }));
      const a = document.createElement("a");
      a.href = url;
      a.download = art.filename || "resume.pdf";
      a.click();
      URL.revokeObjectURL(url);
    };
  } catch {
    button.hidden = true;
  }
}

document.getElementById("settings")!.addEventListener("click", () => {
  $("pair").hidden = !$("pair").hidden;
  ($("server") as HTMLInputElement).value = current.server;
});

document.getElementById("save-pair")!.addEventListener("click", async () => {
  const server = ($("server") as HTMLInputElement).value.trim();
  const token = ($("token") as HTMLInputElement).value.trim();
  if (!server || !token) {
    setError("Server URL and token are both required.");
    return;
  }
  try {
    const origin = new URL(server).origin + "/*";
    await new Promise<void>((resolve) => chrome.permissions.request({ origins: [origin] }, () => resolve()));
  } catch {
    setError("That server URL is not absolute.");
    return;
  }
  await storageSet({ server, token });
  ($("token") as HTMLInputElement).value = "";
  await refresh();
});

document.getElementById("save-job")!.addEventListener("click", async () => {
  const job = current.job;
  if (!job?.title || !job.url) return;
  try {
    const saved = (await api(current.server, current.token, "POST", "/v1/applications/capture", {
      company: job.company,
      title: job.title,
      location: job.location,
      salary: job.salary,
      posting_url: job.url,
    })) as { application: App; created: boolean };
    current.app = saved.application;
    $("note").textContent = saved.created ? "Saved as to apply." : "Already tracked.";
    await refresh();
  } catch (err) {
    setError(err instanceof Error ? err.message : String(err));
  }
});

document.getElementById("status")!.addEventListener("change", async () => {
  const app = current.app;
  if (!app) return;
  const status = ($("status") as HTMLSelectElement).value;
  try {
    const updated = (await api(current.server, current.token, "PATCH", `/v1/applications/${app.id}`, {
      version: app.version,
      status,
    })) as App;
    current.app = updated;
    $("note").textContent = "Status saved.";
  } catch (err) {
    setError(err instanceof Error ? err.message : String(err));
    await refresh();
  }
});

document.getElementById("autofill")!.addEventListener("click", async () => {
  setError("");
  const tab = await activeTab();
  if (!tab?.id) return;
  try {
    const profile = (await api(current.server, current.token, "GET", "/v1/profile")) as { answers: Record<string, string> };
    const answers = { ...(profile.answers || {}) };
    if (current.app) {
      const qs = (await api(current.server, current.token, "GET", `/v1/applications/${current.app.id}/questions`)) as {
        data: { prompt: string; answer: string }[];
      };
      for (const q of qs.data || []) {
        if (!q.answer) continue;
        answers[q.prompt] = q.answer;
        answers[normalizeKey(q.prompt)] = q.answer;
      }
    }
    const report = (await send(tab.id, { type: "autofill", answers })) as FillReport;
    const missed = report?.missed?.length ? ` Could not fill: ${report.missed.join("; ")}.` : "";
    $("note").textContent = `Filled ${report?.filled?.length || 0} field${report?.filled?.length === 1 ? "" : "s"}.${missed} Nothing was submitted.`;
  } catch (err) {
    setError(err instanceof Error ? err.message : String(err));
  }
});

void refresh();
