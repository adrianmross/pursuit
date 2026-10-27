// Pursuit dashboard. Vanilla JS on purpose: the daemon's API is JSON, so the
// page calls it with the bearer token already used by the CLI. No build step.

const TOKEN_KEY = "pursuit.token";

const FILTERS = [
  ["", "All"],
  ["to_apply", "To apply"],
  ["awaiting_review", "Prepared"],
  ["applied", "Applied"],
  ["interviewing", "Interviewing"],
  ["offer", "Offer"],
  ["rejected", "Rejected"],
];

const MORE = [
  ["saved", "Saved"],
  ["preparing", "Preparing"],
  ["needs_input", "Needs input"],
  ["approved", "Approved"],
  ["submitting", "Submitting"],
  ["submitted", "Submitted"],
  ["failed", "Failed"],
  ["in_progress", "In progress"],
  ["screening", "Screening"],
  ["withdrawn", "Withdrawn"],
  ["ghosted", "Ghosted"],
];

const SORTS = [
  ["-updated_at", "Recently updated"],
  ["updated_at", "Least recently updated"],
  ["company", "Company A–Z"],
  ["-company", "Company Z–A"],
  ["title", "Title A–Z"],
  ["status", "Status"],
  ["-created_at", "Newest"],
  ["created_at", "Oldest"],
  ["-applied_date", "Applied date"],
  ["applied_date", "Applied date, earliest"],
];

const LABELS = {
  saved: "Saved",
  to_apply: "To apply",
  preparing: "Preparing",
  awaiting_review: "Prepared",
  approved: "Approved",
  submitting: "Submitting",
  submitted: "Submitted",
  needs_input: "Needs input",
  failed: "Failed",
  in_progress: "In progress",
  applied: "Applied",
  screening: "Screening",
  interviewing: "Interviewing",
  offer: "Offer",
  rejected: "Rejected",
  withdrawn: "Withdrawn",
  ghosted: "Ghosted",
};

const state = {
  token: localStorage.getItem(TOKEN_KEY) || "",
  me: null,
  rows: [],
  next: "",
  total: 0,
  hasMore: false,
};

let loadGen = 0;

const app = document.getElementById("app");

const SVG_NS = "http://www.w3.org/2000/svg";
const SVG_TAGS = new Set(["svg", "path", "circle"]);

function h(tag, props, ...children) {
  const node = SVG_TAGS.has(tag) ? document.createElementNS(SVG_NS, tag) : document.createElement(tag);
  for (const [key, value] of Object.entries(props || {})) {
    if (value == null || value === false) continue;
    if (key === "class") node.className = value;
    else if (key.startsWith("on")) node.addEventListener(key.slice(2).toLowerCase(), value);
    else node.setAttribute(key, value === true ? "" : String(value));
  }
  for (const child of children) add(node, child);
  return node;
}

function add(node, child) {
  if (child == null || child === false) return;
  if (Array.isArray(child)) {
    child.forEach((item) => add(node, item));
    return;
  }
  node.append(child.nodeType ? child : document.createTextNode(String(child)));
}

function can(scope) {
  const scopes = (state.me && state.me.token && state.me.token.scopes) || [];
  return scopes.includes(scope) || (scope === "read" && scopes.includes("write"));
}

function label(status) {
  return LABELS[status] || status || "—";
}

function pillClass(status) {
  if (["offer", "approved", "submitted", "applied"].includes(status)) return "pill pill-ok";
  if (["awaiting_review", "preparing", "needs_input", "interviewing", "screening"].includes(status)) return "pill pill-info";
  if (["rejected", "failed", "ghosted"].includes(status)) return "pill pill-bad";
  if (["withdrawn", "submitting"].includes(status)) return "pill pill-warn";
  return "pill";
}

function linkPill(status) {
  if (status === "closed") return h("span", { class: "pill pill-bad" }, "Dead");
  if (status === "unknown") return h("span", { class: "pill pill-warn" }, "Unknown");
  if (status === "live") return h("span", { class: "pill pill-ok" }, "Live");
  return h("span", { class: "pill" }, "Unchecked");
}

function linkDot(status) {
  const dead = status === "closed";
  const text = dead ? "Dead link" : status === "live" ? "Posting is live" : status === "unknown" ? "Link status unknown" : "Link not checked";
  return h("span", {
    class: dead ? "link-dot link-dot-dead" : "link-dot",
    title: text,
    role: "img",
    "aria-label": text,
  });
}

function postingIcon() {
  return h("svg", { viewBox: "0 0 16 16", width: "15", height: "15", "aria-hidden": "true" },
    h("path", {
      d: "M6.5 3.5H3.8A1.3 1.3 0 0 0 2.5 4.8v7.4A1.3 1.3 0 0 0 3.8 13.5h7.4a1.3 1.3 0 0 0 1.3-1.3V9.5",
      fill: "none", stroke: "currentColor", "stroke-width": "1.4", "stroke-linecap": "round",
    }),
    h("path", {
      d: "M9 2.5h4.5V7",
      fill: "none", stroke: "currentColor", "stroke-width": "1.4", "stroke-linecap": "round", "stroke-linejoin": "round",
    }),
    h("path", {
      d: "M13.5 2.5 7.5 8.5",
      fill: "none", stroke: "currentColor", "stroke-width": "1.4", "stroke-linecap": "round",
    }));
}

function menuGlyph() {
  return h("svg", { viewBox: "0 0 16 16", width: "18", height: "18", "aria-hidden": "true" },
    h("circle", { cx: "3.5", cy: "8", r: "1.15", fill: "currentColor" }),
    h("circle", { cx: "8", cy: "8", r: "1.15", fill: "currentColor" }),
    h("circle", { cx: "12.5", cy: "8", r: "1.15", fill: "currentColor" }));
}

function shortWhen(value) {
  if (!value) return "—";
  const raw = String(value);
  const match = raw.match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (!match) return raw;
  const time = raw.match(/T(\d{2}):(\d{2}):(\d{2})/);
  const date = time
    ? new Date(`${match[1]}-${match[2]}-${match[3]}T${time[1]}:${time[2]}:${time[3]}Z`)
    : new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  if (Number.isNaN(date.getTime())) return match[1] + "-" + match[2] + "-" + match[3];
  const now = new Date();
  const start = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const diff = Math.round((today.getTime() - start.getTime()) / 86400000);
  if (diff === 0) return "today";
  if (diff > 0 && diff < 7) return diff + "d ago";
  const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
  const label = months[date.getMonth()] + " " + date.getDate();
  return date.getFullYear() === now.getFullYear() ? label : label + ", " + date.getFullYear();
}

function rowWhen(row) {
  return shortWhen(row.applied_date || row.updated_at);
}

function fmt(value) {
  if (!value) return "—";
  const match = String(value).match(/^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})/);
  if (!match) return String(value);
  const date = new Date(`${match[1]}T${match[2]}Z`);
  if (Number.isNaN(date.getTime())) return match[1];
  return date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

function safeURL(raw) {
  try {
    const url = new URL(raw);
    if (url.protocol === "http:" || url.protocol === "https:") return url.href;
  } catch {
    /* not a url */
  }
  return "";
}

function parseRoute() {
  const raw = (location.hash || "#/").replace(/^#/, "");
  const cut = raw.indexOf("?");
  const path = cut === -1 ? raw : raw.slice(0, cut);
  const params = new URLSearchParams(cut === -1 ? "" : raw.slice(cut + 1));
  const parts = path.split("/").filter(Boolean);
  if (parts[0] === "app" && parts[1]) return { name: "app", id: decodeURIComponent(parts[1]) };
  if (parts[0] === "queue") return { name: "queue" };
  if (parts[0] === "links") return { name: "links" };
  return {
    name: "pipeline",
    status: params.get("status") || "",
    q: params.get("q") || "",
    sort: params.get("sort") || "-updated_at",
  };
}

function pipelineHash(route) {
  const params = new URLSearchParams();
  if (route.status) params.set("status", route.status);
  if (route.q) params.set("q", route.q);
  if (route.sort && route.sort !== "-updated_at") params.set("sort", route.sort);
  const qs = params.toString();
  return "#/" + (qs ? "?" + qs : "");
}

function currentPipeline(patch) {
  const route = parseRoute();
  const box = document.getElementById("q");
  if (box) route.q = box.value;
  return { ...route, ...patch };
}

async function api(path, options = {}) {
  const headers = {};
  if (state.token) headers.Authorization = "Bearer " + state.token;
  const init = { method: options.method || "GET", headers };
  if (options.body !== undefined) {
    headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(options.body);
  }
  const response = await fetch(path, init);
  const text = await response.text();
  let data = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = { error: { message: text } };
    }
  }
  if (response.status === 401) {
    const hadToken = state.token !== "";
    state.token = "";
    state.me = null;
    localStorage.removeItem(TOKEN_KEY);
    if (hadToken && !options.retried) {
      return api(path, { ...options, retried: true });
    }
    const error = new Error("That token was rejected.");
    error.status = 401;
    throw error;
  }
  if (!response.ok) {
    const body = data && data.error ? data.error : {};
    const error = new Error(body.message || response.statusText || "Request failed");
    error.hint = body.hint || "";
    error.status = response.status;
    throw error;
  }
  return data;
}

function banner(message, hint) {
  const node = document.getElementById("banner");
  if (!node) return;
  node.replaceChildren();
  if (!message) return;
  node.append(message);
  if (hint) node.append(h("div", {}, hint));
}

function shell(route) {
  const links = [
    ["#/", "Pipeline", route.name === "pipeline"],
    ["#/queue", "Queue", route.name === "queue"],
    ["#/links", "Links", route.name === "links"],
  ];
  const identity = state.me
    ? (state.me.token.name || "token") + " · " + (state.me.token.scopes || []).join(", ")
    : "";
  const who = state.me
    ? h("div", { class: "who" },
      h("span", {}, identity),
      h("button", { class: "btn", type: "button", onClick: signOut }, "Sign out"))
    : null;
  const account = state.me
    ? h("details", { class: "account" },
      h("summary", { class: "account-btn", "aria-label": "Account" }, menuGlyph()),
      h("div", { class: "account-pop" },
        h("p", { class: "account-name" }, identity),
        h("button", { class: "btn", type: "button", onClick: signOut }, "Sign out")))
    : null;
  app.replaceChildren(h("div", { class: "shell" },
    h("header", { class: "top" },
      h("a", { class: "brand", href: "#/" }, "Pursuit"),
      h("nav", { class: "nav" }, links.map(([href, text, current]) =>
        h("a", { href, "aria-current": current ? "page" : null }, text))),
      who,
      account),
    h("main", { class: "main", id: "main" },
      h("div", { class: "banner", id: "banner" })),
    h("dialog", { id: "confirm" },
      h("form", { method: "dialog" },
        h("h2", { id: "confirm-title" }, "Confirm"),
        h("p", { id: "confirm-body" }),
        h("label", {}, "Note", h("textarea", { id: "confirm-note", name: "note" })),
        h("div", { class: "actions" },
          h("button", { class: "btn", type: "submit", value: "cancel" }, "Cancel"),
          h("button", { class: "btn btn-primary", id: "confirm-ok", type: "submit", value: "ok" }, "Confirm"))))));
}

function signOut() {
  state.token = "";
  state.me = null;
  localStorage.removeItem(TOKEN_KEY);
  render();
}

function renderSignIn(message) {
  document.title = "Pursuit";
  app.replaceChildren(h("div", { class: "sign-in" },
    h("h1", {}, "Pursuit"),
    h("p", { class: "muted" }, "Paste a token once. It stays in this browser and is sent as a bearer token to this server. When the server trusts the deployment in front of it, this form stays hidden and the page opens as the owner."),
    message ? h("div", { class: "banner" }, message) : null,
    h("form", { onSubmit: onSignIn },
      h("label", {}, "API token",
        h("input", { class: "field", name: "token", type: "password", autocomplete: "off", required: true, autofocus: true })),
      h("button", { class: "btn btn-primary", type: "submit" }, "Continue"),
      h("p", { class: "muted" }, "A read token can browse. Approve and Reject need the approve scope. Re-check needs write. The bootstrap token from the server log has all three."))));
}

async function onSignIn(event) {
  event.preventDefault();
  const token = new FormData(event.target).get("token").trim();
  state.token = token;
  try {
    state.me = await api("/v1/me");
    if (state.token) localStorage.setItem(TOKEN_KEY, state.token);
    render();
  } catch (error) {
    state.token = "";
    state.me = null;
    renderSignIn(error.hint ? error.message + " " + error.hint : error.message);
  }
}

function confirmAction({ title, body, confirm, danger }) {
  const dialog = document.getElementById("confirm");
  document.getElementById("confirm-title").textContent = title;
  document.getElementById("confirm-body").textContent = body;
  document.getElementById("confirm-note").value = "";
  const ok = document.getElementById("confirm-ok");
  ok.textContent = confirm;
  ok.className = danger ? "btn btn-danger" : "btn btn-primary";
  dialog.showModal();
  return new Promise((resolve) => {
    dialog.addEventListener("close", () => {
      resolve(dialog.returnValue === "ok" ? document.getElementById("confirm-note").value : null);
    }, { once: true });
  });
}

async function render() {
  const route = parseRoute();
  const hadToken = state.token !== "";
  if (!state.me) {
    try {
      state.me = await api("/v1/me");
    } catch (error) {
      renderSignIn(hadToken || error.status !== 401 ? error.message : "");
      return;
    }
  }
  shell(route);
  try {
    if (route.name === "app") await renderDetail(route.id);
    else if (route.name === "queue") await renderQueue();
    else if (route.name === "links") await renderLinks();
    else await renderPipeline(route);
  } catch (error) {
    banner(error.message, error.hint);
  }
}

function renderPipeline(route) {
  document.title = "Pipeline · Pursuit";
  const main = document.getElementById("main");
  const statuses = FILTERS.concat(MORE);
  main.append(
    h("div", { class: "page-head" },
      h("div", {},
        h("h1", {}, "Pipeline"),
        h("p", { class: "muted", id: "count" }, "Loading applications")),
      h("div", { class: "toolbar" },
        h("input", {
          class: "field", id: "q", type: "search", placeholder: "Search company, title, notes",
          value: route.q,
          onInput: (event) => {
            const next = currentPipeline({ q: event.target.value });
            window.clearTimeout(state.searchTimer);
            state.searchTimer = window.setTimeout(() => {
              history.replaceState(null, "", pipelineHash(next));
              loadResults(next).catch((error) => banner(error.message, error.hint));
            }, 200);
          },
        }),
        h("select", {
          class: "select", id: "sort", "aria-label": "Sort",
          onChange: (event) => { location.hash = pipelineHash(currentPipeline({ sort: event.target.value })); },
        }, SORTS.map(([value, text]) => h("option", { value, selected: value === route.sort }, text))))),
    h("div", { class: "filters", role: "toolbar", "aria-label": "Status" },
      statuses.map(([value, text]) => h("button", {
        class: "chip", type: "button", "aria-pressed": route.status === value ? "true" : "false",
        onClick: () => { location.hash = pipelineHash(currentPipeline({ status: value })); },
      }, text))),
    h("div", { class: "table-wrap pipeline-wrap" },
      h("table", { class: "pipeline" },
        h("thead", {}, h("tr", {}, [
          ["Company", "col-company"],
          ["Role", "col-role"],
          ["Status", "col-status"],
          ["Link", "col-link"],
          ["Date", "col-date"],
        ].map(([text, cls]) => h("th", {
          class: cls,
          title: text === "Date" ? "Applied date when one is set, otherwise the last update" : null,
        }, text)))),
        h("tbody", { id: "rows" }))),
    h("button", { class: "btn more", id: "more", type: "button", hidden: true, onClick: () => loadResults(parseRoute(), state.next) }, "Load more"));
  return loadResults(route);
}

async function loadResults(route, cursor) {
  const gen = ++loadGen;
  const params = new URLSearchParams();
  params.set("limit", "50");
  if (route.sort) params.set("sort", route.sort);
  if (route.q) params.set("q", route.q);
  if (route.status) params.set("status", route.status);
  if (cursor) params.set("cursor", cursor);
  const page = await api("/v1/applications?" + params.toString());
  if (gen !== loadGen) return;
  if (!cursor) state.rows = [];
  state.rows = state.rows.concat(page.data || []);
  state.total = page.page.total;
  state.hasMore = page.page.has_more;
  state.next = page.page.next_cursor || "";
  const count = document.getElementById("count");
  if (count) count.textContent = state.total === 1 ? "1 application" : state.total + " applications";
  const body = document.getElementById("rows");
  if (!body) return;
  body.replaceChildren();
  if (state.rows.length === 0) {
    body.append(h("tr", { class: "empty-row" }, h("td", { colspan: "5", class: "empty" }, "No applications match.")));
  }
  for (const row of state.rows) body.append(pipelineRow(row));
  const more = document.getElementById("more");
  if (more) more.hidden = !state.hasMore;
}

function pipelineRow(row) {
  const href = safeURL(row.posting_url);
  const detail = "#/app/" + encodeURIComponent(row.id);
  const when = row.applied_date || row.updated_at || "";
  return h("tr", { class: "app-row" },
    h("td", { class: "col-company" }, h("span", { class: "company" }, row.company || "—")),
    h("td", { class: "col-role" },
      h("a", { class: "title-link stretch", href: detail },
        h("span", { class: "role-text" }, row.title || "Untitled"))),
    h("td", { class: "col-status" }, h("span", { class: pillClass(row.status) }, label(row.status))),
    h("td", { class: "col-link" },
      h("span", { class: "link-bits" },
        linkDot(row.link_status),
        href ? h("a", {
          class: "posting-icon", href, target: "_blank", rel: "noopener noreferrer",
          "aria-label": "Open posting",
        }, postingIcon()) : null)),
    h("td", { class: "col-date" },
      h("span", { class: "when", title: row.applied_date ? "Applied " + row.applied_date : when ? "Updated " + when : "" }, rowWhen(row))));
}

async function renderDetail(id) {
  document.title = "Application · Pursuit";
  const [application, events, artifacts, questions] = await Promise.all([
    api("/v1/applications/" + encodeURIComponent(id)),
    api("/v1/applications/" + encodeURIComponent(id) + "/events?limit=100"),
    api("/v1/applications/" + encodeURIComponent(id) + "/artifacts"),
    api("/v1/applications/" + encodeURIComponent(id) + "/questions"),
  ]);
  document.title = (application.title || "Application") + " · Pursuit";
  const posting = safeURL(application.posting_url);
  const main = document.getElementById("main");
  main.append(
    h("p", {}, h("a", { href: "#/" }, "← Pipeline")),
    h("div", { class: "page-head" },
      h("div", {},
        h("h1", {}, application.title || "Untitled"),
        h("p", { class: "muted" }, application.company || "Unknown company")),
      h("div", { class: "actions" },
        h("span", { class: pillClass(application.status) }, label(application.status)),
        application.status === "awaiting_review" ? decisionButtons(application.id) : null)),
    h("section", { class: "card stack" },
      h("div", { class: "meta" },
        posting ? h("a", { href: posting, target: "_blank", rel: "noopener noreferrer" }, "Open posting") : "No posting link",
        linkPill(application.link_status),
        application.location || null,
        application.salary || null,
        application.fit ? "Fit " + application.fit : null,
        application.source || null,
        application.applied_date ? "Applied " + application.applied_date : null),
      application.next_step ? h("p", {}, h("strong", {}, "Next: "), application.next_step) : null,
      (application.tags || []).length ? h("p", {}, "Tags: " + application.tags.join(", ")) : null,
      can("write") ? h("div", {}, h("button", {
        class: "btn", type: "button",
        onClick: () => recheckOne(application.id),
      }, "Re-check link")) : h("p", { class: "muted" }, "Re-checking this posting needs a token with write.")),
    h("section", { class: "card" },
      h("h2", {}, "Notes"),
      application.notes ? h("p", { class: "notes" }, application.notes) : h("p", { class: "muted" }, "No notes.")),
    h("section", { class: "card" },
      h("h2", {}, "Prepared answers"),
      questionList(questions.data || [])),
    h("section", { class: "card" },
      h("h2", {}, "Artifacts"),
      artifactList(artifacts.data || [])),
    h("section", { class: "card" },
      h("h2", {}, "History"),
      eventList(events.data || [])));
}

function decisionButtons(id) {
  if (!can("approve")) {
    return h("span", { class: "muted" }, "Approve needs the approve scope.");
  }
  return [
    h("button", { class: "btn btn-primary", type: "button", onClick: () => decide(id, "approve") }, "Approve"),
    h("button", { class: "btn btn-danger", type: "button", onClick: () => decide(id, "reject") }, "Reject"),
  ];
}

async function decide(id, kind) {
  const approving = kind === "approve";
  const note = await confirmAction({
    title: approving ? "Approve application" : "Reject application",
    body: approving
      ? "Approve marks this application approved. It does not submit the form."
      : "Reject declines this prepared application and marks it withdrawn. Nothing is submitted.",
    confirm: approving ? "Approve" : "Reject",
    danger: !approving,
  });
  if (note == null) return;
  try {
    await api(approving ? "/v1/approvals" : "/v1/approvals/reject", {
      method: "POST",
      body: { ids: [id], note },
    });
    render();
  } catch (error) {
    banner(error.message, error.hint);
  }
}

function questionList(rows) {
  if (!rows.length) return h("p", { class: "muted" }, "No prepared answers yet.");
  return h("div", {}, rows.map((row) => h("div", { class: "question" },
    h("strong", {}, row.prompt || "Question"),
    h("div", {}, row.answer || h("span", { class: "muted" }, "No answer")),
    h("div", { class: "meta" },
      row.required ? "Required" : "Optional",
      row.unknown ? h("span", { class: "pill pill-warn" }, "Needs input") : null,
      row.source || null))));
}

function artifactList(rows) {
  if (!rows.length) return h("p", { class: "muted" }, "No artifacts.");
  return h("div", { class: "stack" }, rows.map((row) => {
    const slot = h("div", { id: "artifact-" + row.id });
    return h("article", {},
      h("div", { class: "actions" },
        h("strong", {}, row.filename || row.kind),
        h("span", { class: "muted" }, row.kind),
        h("button", { class: "btn", type: "button", onClick: () => showArtifact(row, slot) }, "View")),
      slot);
  }));
}

async function showArtifact(row, slot) {
  slot.replaceChildren(h("p", { class: "muted" }, "Loading…"));
  try {
    const art = await api("/v1/artifacts/" + encodeURIComponent(row.id));
    const text = isText(art.content_type);
    const actions = h("div", { class: "actions" },
      h("button", { class: "btn", type: "button", onClick: () => downloadArtifact(art) }, "Download"));
    if (!text) {
      slot.replaceChildren(h("p", { class: "muted" }, "Binary file."), actions);
      return;
    }
    const body = String(art.body || "");
    const shown = body.length > 20000 ? body.slice(0, 20000) + "\n… truncated" : body;
    slot.replaceChildren(h("pre", { class: "artifact" }, shown), actions);
  } catch (error) {
    slot.replaceChildren(h("p", { class: "banner" }, error.message));
  }
}

function isText(contentType) {
  const value = String(contentType || "").toLowerCase();
  return value.startsWith("text/") || value.includes("json") || value === "";
}

function downloadArtifact(art) {
  let blob;
  if (isText(art.content_type)) {
    blob = new Blob([art.body || ""], { type: art.content_type || "text/plain" });
  } else {
    const binary = Uint8Array.from(atob(art.body || ""), (char) => char.charCodeAt(0));
    blob = new Blob([binary], { type: art.content_type || "application/octet-stream" });
  }
  const link = h("a", { href: URL.createObjectURL(blob), download: art.filename || art.kind || "artifact" });
  link.click();
  URL.revokeObjectURL(link.href);
}

function eventList(rows) {
  if (!rows.length) return h("p", { class: "muted" }, "No history yet.");
  return h("ol", { class: "timeline" }, rows.map((row) => h("li", {},
    h("strong", {}, row.title || row.kind),
    h("div", { class: "muted" }, fmt(row.occurred_at), row.actor && row.actor.name ? " · " + row.actor.name : ""),
    row.from_status || row.to_status ? h("div", {}, label(row.from_status), " → ", label(row.to_status)) : null,
    row.body ? h("div", { class: "notes" }, row.body) : null)));
}

async function renderQueue() {
  document.title = "Approval queue · Pursuit";
  const page = await api("/v1/applications?status=awaiting_review&sort=-updated_at&limit=50");
  const rows = page.data || [];
  const main = document.getElementById("main");
  main.append(
    h("div", { class: "page-head" },
      h("div", {},
        h("h1", {}, "Approval queue"),
        h("p", { class: "muted" }, "Agent-prepared applications waiting for you. Approve only records approval."))),
    !can("approve") ? h("div", { class: "banner" }, "This token can view the queue. Approve and Reject need the approve scope.") : null,
    rows.length ? h("div", { class: "stack", id: "queue" }, rows.map(queueCard)) : h("div", { class: "card empty" }, "Nothing is waiting for review."));
  await Promise.all(rows.slice(0, 30).map(async (row) => {
    const slot = document.getElementById("questions-" + row.id);
    if (!slot) return;
    try {
      const questions = await api("/v1/applications/" + encodeURIComponent(row.id) + "/questions");
      slot.replaceChildren(questionList(questions.data || []));
    } catch (error) {
      slot.replaceChildren(h("p", { class: "muted" }, error.message));
    }
  }));
}

function queueCard(row) {
  const posting = safeURL(row.posting_url);
  return h("article", { class: "card stack" },
    h("div", { class: "page-head" },
      h("div", {},
        h("h2", {}, h("a", { class: "title-link", href: "#/app/" + encodeURIComponent(row.id) }, row.title || "Untitled")),
        h("p", { class: "muted" }, row.company || "Unknown company")),
      h("div", { class: "actions" }, decisionButtons(row.id))),
    h("div", { class: "meta" },
      posting ? h("a", { href: posting, target: "_blank", rel: "noopener noreferrer" }, "Posting") : "No posting link",
      linkPill(row.link_status),
      fmt(row.updated_at)),
    row.notes ? h("p", { class: "notes" }, row.notes) : null,
    h("div", { id: "questions-" + row.id }, h("p", { class: "muted" }, "Loading prepared answers…")));
}

async function renderLinks() {
  document.title = "Link health · Pursuit";
  const page = await api("/v1/links?attention=true&limit=100&sort=-checked_at");
  const rows = page.data || [];
  const main = document.getElementById("main");
  main.append(
    h("div", { class: "page-head" },
      h("div", {},
        h("h1", {}, "Link health"),
        h("p", { class: "muted" }, "Postings the checker marked dead, and postings whose status changed since the previous check.")),
      h("button", {
        class: "btn btn-primary", type: "button", id: "recheck-all", disabled: !can("write"),
        onClick: recheckAll,
      }, "Re-check all")),
    !can("write") ? h("p", { class: "muted" }, "Re-check needs a token with the write scope.") : null,
    h("div", { class: "table-wrap" },
      h("table", {},
        h("thead", {}, h("tr", {}, ["Company", "Role", "Result", "Detail", "Checked", ""].map((text) => h("th", {}, text)))),
        h("tbody", { id: "link-rows" }, rows.length ? rows.map(linkRow) : h("tr", {}, h("td", { colspan: "6", class: "empty" }, "No dead or changed postings."))))));
}

function linkRow(row) {
  const posting = safeURL(row.posting_url);
  const result = h("span", {},
    row.status === "closed" ? h("span", { class: "pill pill-bad" }, "Dead") : linkPill(row.status),
    " ",
    row.changed ? h("span", { class: "pill pill-warn" }, "Changed") : null);
  return h("tr", {},
    h("td", { "data-label": "Company" }, row.company || "—"),
    h("td", { "data-label": "Role" }, h("a", { class: "title-link", href: "#/app/" + encodeURIComponent(row.application_id) }, row.title || "Untitled")),
    h("td", { "data-label": "Result" }, result),
    h("td", { "data-label": "Detail" }, row.detail || "—", row.http_status ? h("div", { class: "muted" }, "HTTP " + row.http_status) : null, posting ? h("div", {}, h("a", { href: posting, target: "_blank", rel: "noopener noreferrer" }, "Posting")) : null),
    h("td", { "data-label": "Checked" }, fmt(row.checked_at)),
    h("td", { "data-label": "Action" }, h("button", {
      class: "btn", type: "button", disabled: !can("write"),
      onClick: (event) => recheckOne(row.application_id, event.target),
    }, "Re-check")));
}

async function recheckOne(id, button) {
  if (button) button.disabled = true;
  banner("");
  try {
    await api("/v1/applications/" + encodeURIComponent(id) + "/check-link", { method: "POST", body: {} });
    render();
  } catch (error) {
    if (button) button.disabled = !can("write");
    banner(error.message, error.hint);
  }
}

async function recheckAll(event) {
  const button = event.target;
  button.disabled = true;
  button.textContent = "Checking…";
  banner("");
  try {
    await api("/v1/links/check", { method: "POST", body: { owner: "dashboard" } });
    render();
  } catch (error) {
    button.disabled = false;
    button.textContent = "Re-check all";
    banner(error.message, error.hint);
  }
}

window.addEventListener("hashchange", render);
render();
