// Holds the pairing and makes every API call. A content script runs with the
// job page's origin, so it asks here instead of fetching the server itself.

import { api, type ApiRequest, type Pairing } from "./api";

function pairing(): Promise<Pairing> {
  return new Promise((resolve) =>
    chrome.storage.local.get(["server", "token"], (v) => resolve({ server: v.server || "", token: v.token || "" })),
  );
}

chrome.runtime.onMessage.addListener((msg: ApiRequest & { type?: string }, _sender, sendResponse) => {
  if (msg?.type === "api") {
    pairing()
      .then((p) => api(p, msg.method, msg.path, msg.body))
      .then((data) => sendResponse({ ok: true, data }), (err: Error & { code?: string }) =>
        sendResponse({ ok: false, error: err.message, code: err.code }),
      );
    return true;
  }
  if (msg?.type === "paired") {
    pairing().then((p) => sendResponse({ paired: Boolean(p.server && p.token), server: p.server }));
    return true;
  }
  if (msg?.type === "options") {
    chrome.runtime.openOptionsPage();
  }
});

chrome.commands?.onCommand.addListener((command) => {
  if (command !== "toggle-panel") return;
  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    if (tabs[0]?.id) chrome.tabs.sendMessage(tabs[0].id, { type: "toggle" }, () => void chrome.runtime.lastError);
  });
});
