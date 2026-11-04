// The in-page panel. It shows on job postings and application forms, saves the
// posting, fills the form, and never clicks Submit. A person or an agent driving
// the browser uses the same buttons; the panel's shadow root is open so both can
// read it.

import { autofillDocument, normalizeKey, typedAnswers, type FillReport } from "./autofill";
import { extractJob, type ExtractedJob } from "./extract";
import { LABELS, MANUAL_STATUSES, decodeFile, progressMark, stageOf, tone, type App } from "./api";
import tokens from "./tokens.css";
import panelCSS from "./panel.css";

type Doc = { id: string; kind: string; filename: string; content_type: string; created_at: string };

class ApiError extends Error {
  code?: string;
}

function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  return new Promise((resolve, reject) => {
    chrome.runtime.sendMessage({ type: "api", method, path, body }, (res) => {
      if (chrome.runtime.lastError || !res) return reject(new Error(chrome.runtime.lastError?.message || "Pursuit is reloading. Try again."));
      if (res.ok) return resolve(res.data as T);
      reject(Object.assign(new ApiError(res.error), { code: res.code }));
    });
  });
}

type State = {
  open: boolean;
  job: ExtractedJob;
  form: boolean;
  paired?: boolean;
  server?: string;
  app?: App;
  loading?: boolean;
  busy?: string;
  report?: FillReport;
  note?: string;
  error?: string;
  submitted?: boolean;
};

let state: State = { open: false, job: extractJob(document, location.href), form: hasForm() };
let host: HTMLElement | null = null;
let root: ShadowRoot | null = null;

const ATS = /greenhouse\.io|ashbyhq\.com|lever\.co|myworkdayjobs\.com|smartrecruiters\.com|icims\.com|workable\.com/;

// An application form: an email field beside a file upload, or on a known ATS host.
function hasForm(): boolean {
  const email = document.querySelector("input[type=email], input[name*=email i], input[id*=email i]");
  if (!email) return false;
  return Boolean(document.querySelector('input[type="file"]')) || ATS.test(location.hostname);
}

// The form often lives at the posting URL plus /apply or /application.
function candidates(url: string): string[] {
  const out = [url];
  const bare = url.replace(/\/(apply|application)\/?(?=[?#]|$)/, "");
  if (bare !== url) out.push(bare);
  return out;
}

async function lookup(): Promise<App | undefined> {
  for (const url of candidates(state.job.url)) {
    try {
      return await call<App>("GET", "/v1/applications/by-url?url=" + encodeURIComponent(url));
    } catch (err) {
      if ((err as ApiError).code !== "not_found") throw err;
    }
  }
  return undefined;
}

async function load() {
  state.loading = true;
  state.error = "";
  render();
  const paired = await new Promise<{ paired: boolean; server: string }>((resolve) =>
    chrome.runtime.sendMessage({ type: "paired" }, (r) => resolve(r || { paired: false, server: "" })),
  );
  state.paired = paired.paired;
  state.server = paired.server;
  if (state.paired) {
    try {
      state.app = await lookup();
    } catch (err) {
      state.error = (err as Error).message;
    }
  }
  state.loading = false;
  render();
}

function source(url: string): string {
  const host = new URL(url).hostname;
  for (const s of ["greenhouse", "ashby", "lever", "workday"]) if (host.includes(s === "workday" ? "myworkdayjobs" : s)) return s;
  return "company_site";
}

async function saveJob() {
  await act("save", async () => {
    const job = state.job;
    const saved = await call<{ application: App; created: boolean }>("POST", "/v1/applications/capture", {
      company: job.company,
      title: job.title,
      location: job.location,
      salary: job.salary,
      posting_url: candidates(job.url).at(-1),
      source: source(job.url),
    });
    state.app = saved.application;
    state.note = saved.created ? "Saved as To apply." : "Already tracked.";
  });
}

async function setStatus(status: string) {
  const app = state.app;
  if (!app || status === (app.outcome || app.status)) return;
  await act("status", async () => {
    state.app = await call<App>("PATCH", `/v1/applications/${app.id}`, { version: app.version, status });
    state.note = `Marked ${LABELS[status] || status}.`;
  });
}

// The tailored résumé on this application, else the newest base résumé.
async function resumeFile(): Promise<File | undefined> {
  if (state.app) {
    const arts = await call<{ data: Doc[] }>("GET", `/v1/applications/${state.app.id}/artifacts`);
    const pdf = newest(arts.data?.filter((a) => a.kind === "resume_pdf"));
    if (pdf) {
      const art = await call<{ body: string; filename: string; content_type: string }>("GET", `/v1/artifacts/${pdf.id}`);
      return decodeFile(art.body, art.filename, art.content_type || "application/pdf", true);
    }
  }
  const docs = await call<{ data: Doc[] }>("GET", "/v1/documents");
  const base = newest(docs.data?.filter((d) => d.kind === "base_resume"));
  if (!base) return undefined;
  const doc = await call<{ body: string; filename: string; content_type: string }>("GET", `/v1/documents/${base.id}`);
  const text = /^text\/|json/i.test(doc.content_type || "");
  return decodeFile(doc.body, doc.filename, doc.content_type, !text);
}

function newest<T extends { created_at: string }>(rows?: T[]): T | undefined {
  return rows?.slice().sort((a, b) => b.created_at.localeCompare(a.created_at))[0];
}

async function autofill() {
  await act("fill", async () => {
    const profile = await call<{ answers: Record<string, string> }>("GET", "/v1/profile");
    const answers = { ...(profile.answers || {}) };
    if (state.app) {
      const qs = await call<{ data: { prompt: string; answer: string }[] }>("GET", `/v1/applications/${state.app.id}/questions`);
      for (const q of qs.data || []) {
        if (!q.answer) continue;
        answers[q.prompt] = q.answer;
        answers[normalizeKey(q.prompt)] = q.answer;
      }
    }
    const report = autofillDocument(document, answers, { resume: await resumeFile() });
    state.report = report;
    if (host) {
      host.dataset.pursuitState = "filled";
      host.dataset.pursuitFilled = String(report.filled.length);
      host.dataset.pursuitMissed = String(report.missed.length);
    }
    if (state.app) {
      // The agent running the CLI reads this from the application's events.
      await call("POST", `/v1/applications/${state.app.id}/events`, {
        kind: "note",
        title: `Autofilled ${report.filled.length} field${report.filled.length === 1 ? "" : "s"}${report.missed.length ? `, ${report.missed.length} left` : ""}`,
        body: [
          report.attached ? `Attached ${report.attached}.` : "",
          report.missed.length ? "Left for you: " + report.missed.join("; ") : "Nothing left to answer.",
          "Not submitted.",
        ]
          .filter(Boolean)
          .join("\n"),
      }).catch(() => undefined);
    }
  });
}

async function saveAnswers() {
  await act("answers", async () => {
    const typed = Object.entries(typedAnswers(document));
    if (!typed.length) {
      state.note = "Type your answers into the outlined fields first.";
      return;
    }
    await call("POST", "/v1/batch", {
      requests: typed.map(([key, answer]) => ({ method: "POST", path: "/v1/standing", body: { key, answer } })),
    });
    state.note = `Saved ${typed.length} answer${typed.length === 1 ? "" : "s"} to your profile. Every later form reuses them.`;
  });
}

async function act(name: string, fn: () => Promise<void>) {
  state.busy = name;
  state.error = "";
  state.note = "";
  render();
  try {
    await fn();
  } catch (err) {
    state.error = (err as Error).message;
    if ((err as ApiError).code === "unpaired") state.paired = false;
  }
  state.busy = "";
  render();
}

function h(tag: string, attrs: Record<string, string | boolean | ((e: Event) => void)> = {}, ...kids: (Node | string | false | undefined)[]): HTMLElement {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (typeof v === "function") el.addEventListener(k, v);
    else if (v === true) el.setAttribute(k, "");
    else if (v !== false) el.setAttribute(k, v);
  }
  for (const kid of kids) if (kid) el.append(kid);
  return el;
}

function svg(markup: string): HTMLElement {
  const span = h("span", { "aria-hidden": "false", style: "display:inline-flex" });
  span.innerHTML = markup; // Built only from constants in api.ts.
  return span;
}

function button(label: string, onClick: () => void, opts: { kind?: string; busy?: boolean; disabled?: boolean } = {}) {
  return h(
    "button",
    { type: "button", class: "btn " + (opts.kind || ""), disabled: Boolean(opts.busy || opts.disabled), click: onClick },
    opts.busy ? label + "…" : label,
  );
}

function render() {
  if (!host) mount();
  if (!root) return;
  const style = h("style", {}, tokens + panelCSS);
  if (!state.open) {
    const label = state.form ? "Autofill" : "Save job";
    root.replaceChildren(style, h("button", { type: "button", class: "launcher", "aria-label": `Open Pursuit: ${label}`, click: open }, svg(progressMark(6, "", 22)), label));
    return;
  }
  root.replaceChildren(style, panel());
}

function panel(): HTMLElement {
  const { job, app } = state;
  const head = h(
    "header",
    {},
    svg(progressMark(6, "", 20)),
    h("strong", {}, "Pursuit"),
    button("Close", close, { kind: "ghost" }),
  );
  const body: HTMLElement[] = [];
  if (state.loading) {
    body.push(h("section", {}, h("p", { class: "muted" }, "Checking your tracker…")));
  } else if (!state.paired) {
    body.push(
      h(
        "section",
        {},
        h("p", {}, "Pair this browser with your Pursuit server to save jobs and fill forms."),
        h("div", { class: "row" }, button("Pair", () => chrome.runtime.sendMessage({ type: "options" }))),
      ),
    );
  } else {
    const meta = [job.company, job.location, job.salary].filter(Boolean).join(" · ");
    const shown = app ? app.outcome || app.status : "";
    body.push(
      h(
        "section",
        { "aria-label": "Posting" },
        h("h2", {}, job.title || (state.form ? "Application form" : document.title)),
        meta && h("p", { class: "muted" }, meta),
        app
          ? h(
              "div",
              { class: "row" },
              svg(progressMark(stageOf(app), app.outcome || "", 24)),
              h("span", { class: "chip " + tone(shown) }, LABELS[shown] || shown),
              h("span", { class: "grow" }),
              state.server && h("a", { href: `${state.server.replace(/\/$/, "")}/app/view?id=${encodeURIComponent(app.id)}`, target: "_blank", rel: "noopener" }, "Open in Pursuit"),
            )
          : h("p", { class: "muted" }, "Not tracked yet."),
        app
          ? h("div", { class: "row" }, h("label", { class: "label", for: "status" }, "Status"), statusSelect(app))
          : h("div", { class: "row" }, button("Save job", saveJob, { busy: state.busy === "save", disabled: !job.title })),
      ),
    );
    if (state.form) body.push(fillSection());
  }
  const status = h("p", { role: "status", "aria-live": "polite", class: state.error ? "error" : "muted" }, state.error || state.note || "");
  return h(
    "div",
    { class: "panel", role: "dialog", "aria-label": "Pursuit" },
    head,
    ...body,
    (state.error || state.note) && h("section", {}, status),
    h("footer", {}, "Review before it's sent. Pursuit never clicks Submit."),
  );
}

function statusSelect(app: App): HTMLElement {
  const current = app.outcome || app.status;
  const values = MANUAL_STATUSES.includes(current) ? MANUAL_STATUSES : [current, ...MANUAL_STATUSES];
  const select = h(
    "select",
    { id: "status", class: "input", disabled: state.busy === "status", change: (e) => setStatus((e.target as HTMLSelectElement).value) },
    ...values.map((v) => h("option", { value: v, selected: v === current }, LABELS[v] || v)),
  );
  return select;
}

function fillSection(): HTMLElement {
  const r = state.report;
  const kids: (HTMLElement | false | undefined)[] = [
    h("div", { class: "row" }, button(r ? "Fill again" : "Autofill application", autofill, { busy: state.busy === "fill", kind: r ? "secondary" : "" })),
  ];
  if (r) {
    kids.push(
      h(
        "p",
        {},
        `Filled ${r.filled.length} field${r.filled.length === 1 ? "" : "s"}.`,
        r.attached ? ` Attached ${r.attached}.` : "",
      ),
    );
    if (r.missed.length) {
      kids.push(
        h("p", { class: "muted" }, "Answer these yourself (outlined on the form):"),
        h("ul", {}, ...r.missed.map((m) => h("li", {}, m))),
        h("div", { class: "row" }, button("Save my answers", saveAnswers, { kind: "secondary", busy: state.busy === "answers" })),
      );
    }
  }
  // Approved work belongs to the submit queue; the agent holding it completes it.
  if (state.submitted && state.app && !["approved", "submitting", "submitted", "applied"].includes(state.app.status)) {
    kids.push(h("div", { class: "row" }, h("span", { class: "grow" }, "Sent it?"), button("Mark applied", () => setStatus("applied"), { busy: state.busy === "status" })));
  }
  return h("section", { "aria-label": "Application form" }, ...kids);
}

function open() {
  state.open = true;
  void load();
}

function close() {
  state.open = false;
  render();
}

function fonts() {
  if (document.getElementById("pursuit-fonts")) return;
  const face = (family: string, file: string) =>
    `@font-face{font-family:"${family}";src:url("${chrome.runtime.getURL("fonts/" + file)}") format("woff2");font-weight:100 900;font-display:swap}`;
  const style = document.createElement("style");
  style.id = "pursuit-fonts";
  style.textContent = face("Pursuit Inter", "inter.woff2") + face("Pursuit Outfit", "outfit.woff2");
  document.head?.appendChild(style);
}

function mount() {
  fonts();
  host = document.createElement("div");
  host.setAttribute("data-pursuit-panel", "");
  host.dataset.pursuitState = "idle";
  root = host.attachShadow({ mode: "open" });
  document.documentElement.appendChild(host);
}

function relevant(): boolean {
  return Boolean(state.job.title) || state.form;
}

function refresh() {
  state = { ...state, job: extractJob(document, location.href), form: hasForm() };
  if (relevant() || state.open) {
    render();
    return;
  }
  host?.remove();
  host = null;
  root = null;
}

document.addEventListener("submit", () => {
  state.submitted = true;
  if (state.open) render();
}, true);

chrome.runtime.onMessage.addListener((msg: { type?: string }, _sender, sendResponse) => {
  if (msg?.type === "extract") {
    sendResponse({ job: extractJob(document, location.href), form: hasForm() });
    return;
  }
  if (msg?.type === "toggle" || msg?.type === "open") {
    if (state.open && msg.type === "toggle") close();
    else open();
    sendResponse({ open: state.open });
  }
});

// Single-page boards (LinkedIn, Ashby) render late and change the posting without a reload.
// ponytail: a 1s URL poll; a navigation API listener can replace it once Firefox ships one.
let lastURL = location.href;
let settle = 10;
setInterval(() => {
  if (location.href !== lastURL) {
    lastURL = location.href;
    state.app = undefined;
    state.report = undefined;
    state.submitted = false;
    settle = 10;
    refresh();
    if (state.open) void load();
    return;
  }
  if (!host && settle-- > 0) refresh();
}, 1000);

if (relevant()) render();
