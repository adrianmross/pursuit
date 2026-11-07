// Pairing, and a shortcut to the in-page panel. Saving and filling live on the page.

import { api, type Pairing } from "./api";

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;

function stored(): Promise<Pairing> {
  return new Promise((resolve) =>
    chrome.storage.local.get(["server", "token"], (v) => resolve({ server: v.server || "", token: v.token || "" })),
  );
}

async function show() {
  $("error").textContent = "";
  const p = await stored();
  const paired = Boolean(p.server && p.token);
  $("paired").hidden = !paired;
  $("pair").hidden = paired;
  $<HTMLInputElement>("server").value = p.server;
  if (!paired) return;
  $<HTMLAnchorElement>("dashboard").href = p.server;
  try {
    const { user } = (await api(p, "GET", "/v1/me")) as { user: { name?: string; email?: string } };
    $("who").textContent = `Paired with ${new URL(p.server).host} as ${user?.name || user?.email || "you"}.`;
  } catch (err) {
    $("who").textContent = `Paired with ${p.server}.`;
    $("error").textContent = (err as Error).message;
  }
}

$("pair").addEventListener("submit", async (e) => {
  e.preventDefault();
  const pair = { server: $<HTMLInputElement>("server").value.trim().replace(/\/$/, ""), token: $<HTMLInputElement>("token").value.trim() };
  try {
    new URL(pair.server);
    await api(pair, "GET", "/v1/me");
  } catch (err) {
    $("error").textContent = `Could not pair: ${(err as Error).message}. Check the URL and token, and that the server allows this extension (PURSUIT_CORS_ORIGINS).`;
    return;
  }
  await chrome.storage.local.set(pair);
  $<HTMLInputElement>("token").value = "";
  await show();
});

$("change").addEventListener("click", () => {
  $("pair").hidden = false;
  $("paired").hidden = true;
});

$("show").addEventListener("click", async () => {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab?.id) return;
  chrome.tabs.sendMessage(tab.id, { type: "open" }, () => {
    if (chrome.runtime.lastError) $("error").textContent = "Pursuit cannot reach this page. Reload it and try again.";
    else window.close();
  });
});

void show();
