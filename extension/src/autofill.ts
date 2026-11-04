// Fill the open ATS form. Never clicks Submit.

export type FillReport = {
  filled: string[];
  missed: string[];
  attached?: string;
};

export type FillFiles = { resume?: File };

type Field = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

const ALIASES: [string, string][] = [
  ["e-mail", "email"],
  ["email", "email"],
  ["phone", "phone"],
  ["mobile", "phone"],
  ["first name", "first_name"],
  ["given name", "first_name"],
  ["preferred name", "first_name"],
  ["last name", "last_name"],
  ["family name", "last_name"],
  ["surname", "last_name"],
  ["full name", "name"],
  ["linkedin", "linkedin"],
  ["github", "github"],
  ["gitlab", "gitlab"],
  ["portfolio", "website"],
  ["website", "website"],
  ["authorized to work", "work_authorization"],
  ["authorised to work", "work_authorization"],
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
  if (key === "name" || low.includes("full name")) {
    const full = answers.name || [answers.first_name, answers.last_name].filter(Boolean).join(" ");
    if (full) return full;
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

export function autofillDocument(doc: Document, answers: Record<string, string>, files: FillFiles = {}): FillReport {
  const report: FillReport = { filled: [], missed: [] };
  const fields = [...doc.querySelectorAll("input, textarea, select")] as Field[];
  const groups = new Set<string>();
  for (const el of fields) {
    if (skip(el)) continue;
    const type = (el.getAttribute("type") || "").toLowerCase();
    if (type === "radio") {
      fillRadio(el as HTMLInputElement, answers, groups, report);
      continue;
    }
    // Consent and opt-in boxes are the person's call.
    if (type === "checkbox") continue;
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
    let ok = true;
    if (el.tagName === "SELECT") ok = chooseOption(el as HTMLSelectElement, value);
    else if (el.getAttribute("role") === "combobox") ok = chooseCombobox(el as HTMLInputElement, value);
    else setValue(el as HTMLInputElement | HTMLTextAreaElement, value);
    if (!ok) {
      report.missed.push(label);
      mark(el);
      continue;
    }
    el.classList.remove("pursuit-unfilled");
    report.filled.push(label);
  }
  const uploads = [...doc.querySelectorAll('input[type="file"]')] as HTMLInputElement[];
  for (const input of uploads) {
    const label = fieldLabel(input) || "Résumé";
    if (!/resume|résumé|curriculum|cv\b/i.test(label + " " + (input.name || "") + " " + (input.id || ""))) continue;
    if (files.resume && attach(input, files.resume)) {
      input.classList.remove("pursuit-unfilled");
      report.attached = files.resume.name;
      report.filled.push(label);
      continue;
    }
    report.missed.push(label + " (add a base résumé in Pursuit)");
    mark(input);
  }
  ensureStyle(doc);
  return report;
}

// Values typed into fields autofill marked, keyed for standing answers.
export function typedAnswers(doc: Document): Record<string, string> {
  const out: Record<string, string> = {};
  for (const el of [...doc.querySelectorAll(".pursuit-unfilled")] as Field[]) {
    const type = (el.getAttribute("type") || "").toLowerCase();
    if (type === "file") continue;
    if (type === "radio") {
      const input = el as HTMLInputElement;
      if (!input.checked) continue;
      const box = input.closest("fieldset, [role=radiogroup]");
      const question = groupQuestion(input, box);
      if (question) out[normalizeKey(question)] = fieldLabel(input) || input.value;
      continue;
    }
    const value = el.tagName === "SELECT" ? selectedText(el as HTMLSelectElement) : el.value;
    const key = normalizeKey(fieldLabel(el).replace(/\*/g, ""));
    if (key && value.trim()) out[key] = value.trim();
  }
  return out;
}

function selectedText(el: HTMLSelectElement): string {
  const opt = el.options[el.selectedIndex];
  return opt && opt.value ? clean(opt.text || opt.textContent || "") : "";
}

// A file input takes a FileList built through DataTransfer, the same path a drop uses.
function attach(input: HTMLInputElement, file: File): boolean {
  if (typeof DataTransfer === "undefined") return false;
  try {
    const dt = new DataTransfer();
    dt.items.add(file);
    input.files = dt.files;
  } catch {
    return false;
  }
  fire(input, "input");
  fire(input, "change");
  return (input.files?.length ?? 0) > 0;
}

function groupQuestion(el: HTMLInputElement, box: Element | null): string {
  const labelled = box?.getAttribute("aria-labelledby") || "";
  return clean(
    box?.querySelector("legend")?.textContent ||
      (labelled && el.ownerDocument?.getElementById(labelled)?.textContent) ||
      box?.getAttribute("aria-label") ||
      el.name.replace(/[_[\]]+/g, " "),
  );
}

// A radio group answers its question (legend or radiogroup label) with the option whose label matches.
function fillRadio(el: HTMLInputElement, answers: Record<string, string>, groups: Set<string>, report: FillReport): void {
  const box = el.closest("fieldset, [role=radiogroup]");
  const group = el.name || box?.id || "";
  if (!group || groups.has(group)) return;
  groups.add(group);
  const question = groupQuestion(el, box);
  if (!question) return;
  const scope = box || el.ownerDocument;
  const options = ([...scope.querySelectorAll("input[type=radio]")] as HTMLInputElement[]).filter(
    (r) => !el.name || r.name === el.name,
  );
  const value = matchAnswer(question, answers).toLowerCase();
  const hit = value
    ? options.find((r) => fieldLabel(r).toLowerCase() === value || (r.getAttribute("value") || "").toLowerCase() === value)
    : undefined;
  if (!hit) {
    if (options.some((r) => r.required) || question.includes("*")) {
      report.missed.push(question);
      options.forEach(mark);
    }
    return;
  }
  hit.checked = true;
  fire(hit, "click");
  fire(hit, "change");
  options.forEach((r) => r.classList.remove("pursuit-unfilled"));
  report.filled.push(question);
}

// A typeahead select (react-select on Greenhouse and Ashby): type the answer, then pick the option it lists.
// ponytail: picks only options rendered synchronously after input; async-loaded lists (city search) stay missed.
function chooseCombobox(el: HTMLInputElement, value: string): boolean {
  el.focus?.();
  setValue(el, value);
  const doc = el.ownerDocument;
  const listId = el.getAttribute("aria-controls") || el.getAttribute("aria-owns") || "";
  const list = (listId && doc.getElementById(listId)) || doc;
  const want = value.toLowerCase();
  const options = [...list.querySelectorAll("[role=option]")] as HTMLElement[];
  const text = (o: HTMLElement) => clean(o.textContent || "").toLowerCase();
  const opt = options.find((o) => text(o) === want) || options.find((o) => text(o).includes(want));
  if (!opt) return false;
  fire(opt, "mousedown");
  opt.click?.();
  return true;
}

function skip(el: Field): boolean {
  const type = (el.getAttribute("type") || "").toLowerCase();
  if (["hidden", "submit", "button", "password", "image", "reset", "file"].includes(type)) return true;
  if (el.hasAttribute("hidden") || el.getAttribute("aria-hidden") === "true") return true;
  if (el.closest("[data-pursuit-panel]")) return true;
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
  const by = el.getAttribute("aria-labelledby");
  if (by) {
    const text = clean(el.ownerDocument?.getElementById(by)?.textContent || "");
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
  // Needs input is amber in the design system.
  style.textContent = ".pursuit-unfilled { outline: 2px solid #f59e0b !important; outline-offset: 2px; }";
  doc.documentElement?.appendChild(style);
}
