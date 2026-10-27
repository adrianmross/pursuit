import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { parseHTML } from "linkedom";
import { autofillDocument, matchAnswer, normalizeKey } from "../src/autofill.ts";

function form(file: string) {
  const html = readFileSync(new URL("../fixtures/" + file, import.meta.url), "utf8");
  return parseHTML(html).document;
}

const profile = {
  email: "person@example.com",
  phone: "555-0100",
  first_name: "Ada",
  last_name: "Lovelace",
  name: "Ada Lovelace",
  work_authorization: "Yes",
  cover_letter: "I build reliable systems.",
  website: "https://example.com",
  why_us: "Standing answer that a prepared prompt should replace.",
};

test("prepared prompts match by the same key as standing answers", () => {
  assert.equal(normalizeKey("Why us?"), "why_us");
  const answers = { ...profile, "Why us?": "Prepared for this role.", why_us: "Prepared for this role." };
  assert.equal(matchAnswer("Why us?", answers), "Prepared for this role.");
  assert.equal(matchAnswer("Email*", profile), "person@example.com");
  assert.equal(matchAnswer("Full name", profile), "Ada Lovelace");
});

test("Greenhouse form fills contact and prepared answers and never submits", () => {
  const doc = form("greenhouse-form.html");
  let submitted = 0;
  doc.getElementById("submit").addEventListener("click", () => {
    submitted += 1;
  });
  const report = autofillDocument(doc, {
    ...profile,
    "Why us?": "Prepared for this role.",
    why_us: "Prepared for this role.",
  });
  assert.equal(submitted, 0);
  assert.equal((doc.getElementById("first_name") as HTMLInputElement).value, "Ada");
  assert.equal((doc.getElementById("email") as HTMLInputElement).value, "person@example.com");
  assert.equal((doc.querySelector("textarea") as HTMLTextAreaElement).value, "Prepared for this role.");
  assert.equal((doc.getElementById("auth") as HTMLSelectElement).value, "yes");
  assert.equal((doc.getElementById("resume") as HTMLInputElement).files?.length ?? 0, 0);
  assert.ok(doc.getElementById("linkedin").classList.contains("pursuit-unfilled"));
  assert.ok(doc.getElementById("resume").classList.contains("pursuit-unfilled"));
  assert.ok(report.filled.some((label) => label.startsWith("Email")));
  assert.ok(report.missed.some((label) => label.startsWith("LinkedIn")));
  assert.ok(report.missed.some((label) => /prepared PDF/.test(label)));
  assert.equal(doc.getElementById("phone").classList.contains("pursuit-unfilled"), false);
});

test("Ashby form fills name, email, and the prepared prompt", () => {
  const doc = form("ashby-form.html");
  let submitted = 0;
  doc.querySelector("button")!.addEventListener("click", () => {
    submitted += 1;
  });
  const report = autofillDocument(doc, { ...profile, "Why Ashby?": "I like the product." });
  assert.equal(submitted, 0);
  assert.equal((doc.getElementById("name") as HTMLInputElement).value, "Ada Lovelace");
  assert.equal((doc.getElementById("email") as HTMLInputElement).value, "person@example.com");
  assert.equal((doc.getElementById("why") as HTMLTextAreaElement).value, "I like the product.");
  assert.equal((doc.getElementById("website") as HTMLInputElement).value, "https://example.com");
  assert.ok(report.missed.some((label) => /prepared PDF/.test(label)));
});

test("Lever form fills wrapped labels and leaves the file input empty", () => {
  const doc = form("lever-form.html");
  const report = autofillDocument(doc, profile);
  const inputs = [...doc.querySelectorAll("input")] as HTMLInputElement[];
  assert.equal(inputs.find((el) => el.name === "name")!.value, "Ada Lovelace");
  assert.equal(inputs.find((el) => el.name === "email")!.value, "person@example.com");
  assert.equal(inputs.find((el) => el.name === "phone")!.value, "555-0100");
  assert.equal((doc.querySelector("textarea") as HTMLTextAreaElement).value, "I build reliable systems.");
  assert.equal(inputs.find((el) => el.type === "file")!.value, "");
  assert.ok(report.missed.some((label) => /prepared PDF/.test(label)));
  assert.equal(doc.querySelectorAll("button[type=submit]").length, 1);
});
