// Runs on every page and frame, so it stays small: it only checks whether the
// page is a posting or a form, then loads the panel (React, panel.js) there.

import { relevant } from "./detect";

type PanelAPI = { toggle(): void; open(): void; refresh(): void };

let panel: Promise<PanelAPI> | null = null;

function load(): Promise<PanelAPI> {
  panel ??= import(chrome.runtime.getURL("panel.js")).then((m: { mount: () => PanelAPI }) => m.mount());
  return panel;
}

chrome.runtime.onMessage.addListener((msg: { type?: string }, _sender, sendResponse) => {
  if (msg?.type !== "toggle" && msg?.type !== "open") return;
  void load().then((p) => {
    if (msg.type === "toggle") p.toggle();
    else p.open();
    sendResponse({ ok: true });
  });
  return true;
});

// Single-page boards (LinkedIn, Ashby) render late and change the posting without a reload.
// ponytail: a 1s URL poll; a navigation API listener can replace it once Firefox ships one.
let lastURL = location.href;
let settle = 10;
setInterval(() => {
  if (location.href !== lastURL) {
    lastURL = location.href;
    settle = 10;
    if (panel) void panel.then((p) => p.refresh());
  }
  if (!panel && settle-- > 0 && relevant()) void load();
}, 1000);

if (relevant()) void load();
