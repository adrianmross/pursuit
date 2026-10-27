// Read the posting the user already has open. No network.

export type ExtractedJob = {
  company: string;
  title: string;
  location: string;
  salary: string;
  url: string;
};

type Doc = Pick<Document, "querySelector" | "querySelectorAll">;

export function extractJob(doc: Doc, pageURL: string): ExtractedJob {
  const fromSchema = readJobPosting(doc);
  const fromSite = readSite(doc, pageURL);
  const job: ExtractedJob = {
    company: fromSchema.company || fromSite.company || "",
    title: fromSchema.title || fromSite.title || "",
    location: fromSchema.location || fromSite.location || "",
    salary: fromSchema.salary || fromSite.salary || "",
    url: pageURL,
  };
  return job;
}

function readSite(doc: Doc, pageURL: string): Partial<ExtractedJob> {
  let host = "";
  let path = "";
  try {
    const u = new URL(pageURL);
    host = u.hostname.toLowerCase();
    path = u.pathname;
  } catch {
    return {};
  }
  if (host.includes("greenhouse.io")) return greenhouse(doc, path);
  if (host.includes("ashbyhq.com")) return ashby(doc, path);
  if (host.includes("lever.co")) return lever(doc, path);
  if (host.includes("myworkdayjobs.com")) return workday(doc, host);
  if (host.includes("linkedin.com") && path.includes("/jobs/")) return linkedin(doc);
  return {};
}

function text(doc: Doc, selector: string): string {
  return clean(doc.querySelector(selector)?.textContent || "");
}

function clean(s: string): string {
  return s.replace(/\s+/g, " ").trim();
}

function greenhouse(doc: Doc, path: string): Partial<ExtractedJob> {
  const board = path.split("/").filter(Boolean)[0] || "";
  return {
    title: text(doc, "h1"),
    location: text(doc, ".location, .job__location"),
    company: text(doc, ".company-name") || titleCase(board.replace(/-/g, " ")),
    salary: salaryText(text(doc, ".salary, .compensation")),
  };
}

function ashby(doc: Doc, path: string): Partial<ExtractedJob> {
  const board = path.split("/").filter(Boolean)[0] || "";
  return {
    title: text(doc, "h1"),
    location: text(doc, ".ashby-job-posting-location, [class*='location']"),
    company: titleCase(board.replace(/-/g, " ")),
  };
}

function lever(doc: Doc, path: string): Partial<ExtractedJob> {
  const site = path.split("/").filter(Boolean)[0] || "";
  const cats = [...doc.querySelectorAll(".posting-category")].map((n) => clean(n.textContent || ""));
  return {
    title: text(doc, ".posting-headline h2, .posting-headline h1"),
    location: cats[0] || "",
    company: titleCase(site.replace(/-/g, " ")),
    salary: cats.map(salaryText).find(Boolean) || "",
  };
}

function workday(doc: Doc, host: string): Partial<ExtractedJob> {
  const sub = host.split(".")[0] || "";
  return {
    title: text(doc, "[data-automation-id='jobPostingHeader']"),
    location: text(doc, "[data-automation-id='locations'], [data-automation-id='job-location']"),
    company: titleCase(sub.replace(/-/g, " ")),
    salary: salaryText(text(doc, "[data-automation-id='compensation']")),
  };
}

function linkedin(doc: Doc): Partial<ExtractedJob> {
  const insights = [...doc.querySelectorAll(".job-details-jobs-unified-top-card__job-insight, .salary")].map((n) =>
    clean(n.textContent || ""),
  );
  return {
    title: text(doc, ".job-details-jobs-unified-top-card__job-title, h1.topcard__title, h1"),
    company: text(doc, ".job-details-jobs-unified-top-card__company-name, .topcard__org-name-link"),
    location: text(doc, ".job-details-jobs-unified-top-card__primary-description-container, .topcard__flavor--bullet"),
    salary: insights.map(salaryText).find(Boolean) || "",
  };
}

function titleCase(s: string): string {
  return clean(s).replace(/\b[a-z]/g, (c) => c.toUpperCase());
}

function salaryText(s: string): string {
  const m = s.match(/\$[\d,.]+[kK]?(?:\s*[–-]\s*\$[\d,.]+[kK]?)?/);
  return m ? m[0].replace(/\s+/g, "") : "";
}

function readJobPosting(doc: Doc): Partial<ExtractedJob> {
  const scripts = doc.querySelectorAll('script[type="application/ld+json"]');
  for (const script of scripts) {
    const found = findPosting(parseJSON(script.textContent || ""));
    if (found) return found;
  }
  return {};
}

function parseJSON(raw: string): unknown {
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

function findPosting(node: unknown): Partial<ExtractedJob> | null {
  if (!node || typeof node !== "object") return null;
  if (Array.isArray(node)) {
    for (const item of node) {
      const found = findPosting(item);
      if (found) return found;
    }
    return null;
  }
  const obj = node as Record<string, unknown>;
  const types = obj["@type"];
  const typeList = Array.isArray(types) ? types : [types];
  if (typeList.includes("JobPosting")) return postingFields(obj);
  if (Array.isArray(obj["@graph"])) return findPosting(obj["@graph"]);
  return null;
}

function postingFields(obj: Record<string, unknown>): Partial<ExtractedJob> {
  const org = obj.hiringOrganization;
  let company = "";
  if (typeof org === "string") company = org;
  else if (org && typeof org === "object" && typeof (org as Record<string, unknown>).name === "string") {
    company = (org as Record<string, unknown>).name as string;
  }
  return {
    title: typeof obj.title === "string" ? clean(obj.title) : "",
    company: clean(company),
    location: locationText(obj.jobLocation),
    salary: moneyText(obj.baseSalary),
  };
}

function locationText(node: unknown): string {
  if (typeof node === "string") return clean(node);
  if (Array.isArray(node)) return locationText(node[0]);
  if (!node || typeof node !== "object") return "";
  const obj = node as Record<string, unknown>;
  const address = obj.address;
  if (typeof address === "string") return clean(address);
  if (address && typeof address === "object") {
    const a = address as Record<string, unknown>;
    return clean([a.addressLocality, a.addressRegion, a.addressCountry].filter((p) => typeof p === "string").join(", "));
  }
  return "";
}

function moneyText(node: unknown): string {
  if (!node || typeof node !== "object") return "";
  const value = (node as Record<string, unknown>).value;
  if (!value || typeof value !== "object") return "";
  const v = value as Record<string, unknown>;
  const min = num(v.minValue);
  const max = num(v.maxValue);
  const one = num(v.value);
  if (min && max) return `${dollars(min)}–${dollars(max)}`;
  if (one) return dollars(one);
  if (min) return dollars(min);
  return "";
}

function num(v: unknown): number {
  return typeof v === "number" ? v : 0;
}

function dollars(n: number): string {
  if (n >= 1000 && n % 1000 === 0) return `$${n / 1000}K`;
  return `$${n.toLocaleString("en-US")}`;
}
