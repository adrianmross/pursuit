// Fill the open ATS form. Never clicks Submit.

export type FillReport = {
  filled: string[];
  missed: string[];
};

type Field = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

const ALIASES: [string, string][] = [
  ["e-mail", "email"],
  ["email", "email"],
  ["phone", "phone"],
  ["mobile", "phone"],
  ["first name", "first_name"],
  ["given name", "first_name"],
  ["last name", "last_name"],
  ["family name", "last_name"],
  ["full name", "name"],
  ["linkedin", "linkedin"],
  ["github", "github"],
  ["portfolio", "website"],
  ["website", "website"],
  ["authorized to work", "work_authorization"],
  ["work authorization", "work_authorization"],
  ["sponsorship", "visa_sponsorship"],
  ["visa", "visa_sponsorship"],
  ["pronouns", "pronouns"],
  ["cover letter", "cover_letter"],
  ["location", "location"],
  ["city", "location"],
];

export function normalizeKey(s: string): string {
  return s
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_|_$/g, "");
}

export function matchAnswer(label: string, answers: Record<string, string>): string {
  const key = normalizeKey(label);
  if (answers[key]) return answers[key];
  const low = label.toLowerCase();
  if ((low === "name" || key === "name") && answers.name) return answers.name;
  if ((low === "name" || key === "name") && answers.first_name) {
    return [answers.first_name, answers.last_name].filter(Boolean).join(" ");
  }
  for (const [needle, alias] of ALIASES) {
    if (low.includes(needle) && answers[alias]) return answers[alias];
  }
  let hit = "";
  for (const [k, v] of Object.entries(answers)) {
    if (!v) continue;
    const nk = normalizeKey(k);
    if (nk === key) {
      hit = v;
      continue;
    }
    const words = nk.replace(/_/g, " ");
    if (nk.length >= 8 && low.includes(words)) hit = v;
  }
  return hit;
}

export function autofillDocument(doc: Document, answers: Record<string, string>): FillReport {
  const report: FillReport = { filled: [], missed: [] };
  const fields = [...doc.querySelectorAll("input, textarea, select")] as Field[];
  for (const el of fields) {
    if (skip(el)) continue;
    const label = fieldLabel(el);
    if (!label) continue;
    const value = matchAnswer(label, answers);
    if (!value) {
      if (required(el, label)) {
        report.missed.push(label);
        mark(el);
      }
      continue;
    }
    if (el.tagName === "SELECT") {
      if (!chooseOption(el as HTMLSelectElement, value)) {
        report.missed.push(label);
        mark(el);
        continue;
      }
    } else {
      setValue(el as HTMLInputElement | HTMLTextAreaElement, value);
    }
    el.classList.remove("pursuit-unfilled");
    report.filled.push(label);
  }
  const resume = [...doc.querySelectorAll('input[type="file"]')] as HTMLInputElement[];
  for (const input of resume) {
    const label = fieldLabel(input) || "Résumé";
    if (!/resume|curriculum|cv\b/i.test(label + " " + (input.name || "") + " " + (input.id || ""))) continue;
    report.missed.push(label + " — download the prepared PDF from the popup");
    mark(input);
  }
  ensureStyle(doc);
  return report;
}

function skip(el: Field): boolean {
  const type = (el.getAttribute("type") || "").toLowerCase();
  if (["hidden", "submit", "button", "password", "image", "reset", "file"].includes(type)) return true;
  if (el.hasAttribute("hidden") || el.getAttribute("aria-hidden") === "true") return true;
  return false;
}

function required(el: Field, label: string): boolean {
  return el.required || el.getAttribute("aria-required") === "true" || label.includes("*");
}

function fieldLabel(el: Field): string {
  const id = el.getAttribute("id");
  if (id) {
    const labelled = el.ownerDocument?.querySelector(`label[for="${cssEscape(id)}"]`);
    const text = clean(labelled?.textContent || "");
    if (text) return text;
  }
  const parent = el.closest("label");
  if (parent) {
    const clone = parent.cloneNode(true) as HTMLElement;
    clone.querySelectorAll("input, textarea, select").forEach((n) => n.remove());
    const text = clean(clone.textContent || "");
    if (text) return text;
  }
  const aria = el.getAttribute("aria-label") || "";
  if (aria) return clean(aria);
  const name = el.getAttribute("name") || "";
  const bracket = name.match(/\[([^\]]+)\]/);
  if (bracket) return bracket[1].replace(/_/g, " ");
  return clean(el.getAttribute("placeholder") || name || id || "");
}

function cssEscape(s: string): string {
  if (typeof CSS !== "undefined" && CSS.escape) return CSS.escape(s);
  return s.replace(/[^a-zA-Z0-9_-]/g, "\\$&");
}

function clean(s: string): string {
  return s.replace(/\s+/g, " ").trim();
}

function setValue(el: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  const proto = Object.getPrototypeOf(el);
  const desc = Object.getOwnPropertyDescriptor(proto, "value");
  if (desc?.set) {
    try {
      desc.set.call(el, value);
    } catch {
      el.value = value;
    }
  } else {
    el.value = value;
  }
  fire(el, "input");
  fire(el, "change");
}

// Chrome accepts a platform Event. linkedom's dispatcher writes eventPhase, which a platform Event rejects.
function fire(el: Element, type: string): void {
  try {
    el.dispatchEvent(new Event(type, { bubbles: true }));
  } catch {
    el.dispatchEvent({
      type,
      bubbles: true,
      eventPhase: 0,
      CAPTURING_PHASE: 1,
      AT_TARGET: 2,
      BUBBLING_PHASE: 3,
      NONE: 0,
      _path: [],
      cancelBubble: false,
      defaultPrevented: false,
    } as unknown as Event);
  }
}

function chooseOption(el: HTMLSelectElement, value: string): boolean {
  const want = value.toLowerCase();
  for (let i = 0; i < el.options.length; i++) {
    const opt = el.options[i];
    const raw = opt.getAttribute("value") ?? opt.value ?? "";
    const text = (opt.text || opt.textContent || "").toLowerCase();
    if (raw.toLowerCase() === want || text === want || (want !== "" && text.includes(want))) {
      opt.selected = true;
      try {
        el.value = raw;
      } catch {
        // Some DOM implementations derive value from the selected option.
      }
      fire(el, "change");
      return true;
    }
  }
  return false;
}

function mark(el: Element): void {
  el.classList.add("pursuit-unfilled");
}

function ensureStyle(doc: Document): void {
  if (doc.getElementById("pursuit-unfilled-style")) return;
  const style = doc.createElement("style");
  style.id = "pursuit-unfilled-style";
  style.textContent = ".pursuit-unfilled { outline: 2px solid #b45309; outline-offset: 2px; }";
  doc.documentElement?.appendChild(style);
}
