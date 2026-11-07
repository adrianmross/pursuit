// `npm run try`: a fresh server, a seeded profile, and a visible Chromium with
// the extension paired, opened on a tour of demo ATS pages. Close the browser
// or press Ctrl+C to throw it all away.

import { OWNER, fixture, start } from "./harness.ts";

const h = await start({ headless: false, port: Number(process.env.PORT) || 8799 });
await h.pair();

// One posting the agent can work on right away: prepared, approved, waiting in the queue.
const { application: queued } = await h.call("POST", "/v1/applications/capture", {
  company: "Acme Robotics",
  title: "Platform Engineer",
  posting_url: "https://job-boards.greenhouse.io/acme/jobs/222",
  source: "greenhouse",
});
h.cli("apply", queued.id, "--prepare");
await h.call("POST", "/v1/approvals", { ids: [queued.id] });

const greenhouse = fixture("greenhouse-apply.html");
const platform = greenhouse.replaceAll("Backend Engineer", "Platform Engineer");
const pages: Record<string, string> = {
  "https://jobs.lever.co/acme/333": fixture("lever-posting.html"),
  "https://jobs.lever.co/acme/333/apply": fixture("lever-form.html"),
  "https://careers.acme.example/jobs/backend": `<!doctype html><title>Careers at Acme</title>
    <body style="font:16px system-ui;margin:40px;max-width:900px">
    <h1>Careers at Acme Robotics</h1><p>Our job board is embedded below, the way many company sites do it.</p>
    <iframe src="https://job-boards.greenhouse.io/acme/jobs/444" style="width:100%;height:720px;border:1px solid #ccc"></iframe>`,
};

await h.context.route("https://job-boards.greenhouse.io/acme/**", (route) =>
  route.fulfill({ contentType: "text/html", body: route.request().url().includes("/222") ? platform : greenhouse }),
);
for (const [url, body] of Object.entries(pages)) {
  await h.context.route(url, (route) => route.fulfill({ contentType: "text/html", body }));
}
await h.context.route("https://tour.pursuit.example/", (route) =>
  route.fulfill({ contentType: "text/html", body: tour(h.base, queued.id) }),
);

const page = h.context.pages()[0] || (await h.context.newPage());
await page.goto("https://tour.pursuit.example/");

const agentEnv = `export PURSUIT_SERVER=${h.base} PURSUIT_TOKEN=${h.agentToken}`;
console.log(`
Pursuit extension tour is open in Chromium.

  Dashboard   ${h.base}   (sign in with the owner token below)
  Owner token ${OWNER}   (read, write, approve: you)
  Agent token ${h.agentToken}   (read, write: the agent)

Play the agent from another terminal, using this build's CLI:

  ${agentEnv}
  ${h.bin} queue claim --worker agent-1
  ${h.bin} queue complete WORK_ID --worker agent-1 --status submitted

To try it in your own Chrome instead: chrome://extensions, Developer mode,
Load unpacked, pick extension/dist, then pair with ${h.base} and the agent token.

Close the browser or press Ctrl+C to stop. Nothing is kept.
`);

let stopping = false;
async function stop() {
  if (stopping) return;
  stopping = true;
  await h.stop();
  process.exit(0);
}
h.context.on("close", stop);
process.on("SIGINT", stop);

function tour(server: string, queuedID: string): string {
  const step = (n: number, title: string, url: string, body: string) => `
    <li><h2>${n}. ${title}</h2><p>${body}</p><p><a href="${url}">${url.replace("https://", "")}</a></p></li>`;
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Pursuit extension tour</title>
  <style>
    body{margin:0;background:#f6f6f4;color:#141615;font:15px/1.55 system-ui,sans-serif}
    main{max-width:720px;margin:0 auto;padding:40px 20px}
    h1{font-size:28px;margin:0 0 6px} h2{font-size:17px;margin:0 0 4px}
    ol{list-style:none;padding:0} li{background:#fff;border:1px solid #dcdcd6;border-radius:10px;padding:16px 20px;margin:0 0 16px}
    p{margin:4px 0} a{color:#141615} code{font:13px ui-monospace,Menlo,monospace;background:#ececE8;padding:1px 4px;border-radius:4px}
    .muted{color:#5d615e}
  </style></head><body><main>
  <h1>Try the Pursuit extension</h1>
  <p class="muted">These pages are local copies of real ATS layouts, served at their real addresses. Your profile is Ada Lovelace with a résumé already uploaded. Open <a href="${server}">the dashboard</a> in another tab to watch applications and events appear.</p>
  <ol>
  ${step(1, "Save a posting, then fill its form", "https://job-boards.greenhouse.io/acme/jobs/111", "Click the <b>hire pursuit</b> tab on the right edge. Save job, then Autofill this form: contact fields, the Yes/No radios, the sponsorship typeahead, and the résumé upload all fill. <i>Why Acme Robotics?</i> stays outlined: type an answer and press <b>Save my answer</b>. Submit the form and <b>Mark applied</b> appears. Nothing is ever submitted for you.")}
  ${step(2, "A form on a separate /apply page", "https://jobs.lever.co/acme/333", "Save the posting here, then open <a href=\"https://jobs.lever.co/acme/333/apply\">the /apply page</a>: the panel already knows it is tracked. Full name and the cover letter come from your profile.")}
  ${step(3, "A form embedded on a company site", "https://careers.acme.example/jobs/backend", "The Greenhouse form sits in an iframe; the Pursuit button appears inside it.")}
  ${step(4, "Hand it to an agent", "https://job-boards.greenhouse.io/acme/jobs/222", `<i>Platform Engineer</i> is prepared and approved. In a terminal, run the agent commands printed by <code>npm run try</code>: <code>queue claim</code> returns this posting. Open it, Autofill (as an agent driving Chrome would), submit, then <code>queue complete … --status submitted</code>. The panel shows the status move and the dashboard lists the autofill event. Application id: <code>${queuedID}</code>.`)}
  ${step(5, "Keyboard and dark mode", "https://job-boards.greenhouse.io/acme/jobs/555", "Alt+Shift+P opens and closes the panel. Switch your OS to dark mode and the panel follows.")}
  </ol></main></body></html>`;
}
