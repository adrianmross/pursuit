import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { parseHTML } from "linkedom";
import { extractJob } from "../src/extract.ts";

function page(file: string, url: string) {
  const html = readFileSync(new URL("../fixtures/" + file, import.meta.url), "utf8");
  const { document } = parseHTML(html);
  return extractJob(document, url);
}

test("JSON-LD JobPosting wins over the visible heading", () => {
  const job = page(
    "jsonld-job.html",
    "https://job-boards.greenhouse.io/northwind/jobs/42?utm_source=linkedin#apply",
  );
  assert.equal(job.title, "Staff Engineer");
  assert.equal(job.company, "Northwind");
  assert.equal(job.location, "Seattle, WA, US");
  assert.equal(job.salary, "$140K–$200K");
  assert.equal(job.url, "https://job-boards.greenhouse.io/northwind/jobs/42?utm_source=linkedin#apply");
});

test("Greenhouse fills salary when the schema omits it", () => {
  const job = page(
    "greenhouse-posting.html",
    "https://job-boards.greenhouse.io/acme-robotics/jobs/111",
  );
  assert.equal(job.title, "Backend Engineer");
  assert.equal(job.company, "Acme Robotics");
  assert.equal(job.location, "New York, NY");
  assert.equal(job.salary, "$160,000-$190,000");
});

test("Ashby reads the board slug and location", () => {
  const job = page(
    "ashby-posting.html",
    "https://jobs.ashbyhq.com/northwind/abc12345-aaaa-bbbb-cccc-ddddeeeeffff",
  );
  assert.equal(job.title, "Product Engineer");
  assert.equal(job.company, "Northwind");
  assert.equal(job.location, "Remote — United States");
  assert.equal(job.salary, "");
});

test("Lever reads the headline and categories", () => {
  const job = page("lever-posting.html", "https://jobs.lever.co/block-inc/11111111-2222-3333-4444-555555555555");
  assert.equal(job.title, "Support Engineer");
  assert.equal(job.company, "Block Inc");
  assert.equal(job.location, "San Francisco");
  assert.equal(job.salary, "$140K–$180K");
});

test("Workday reads automation ids and the tenant subdomain", () => {
  const job = page(
    "workday-posting.html",
    "https://block.wd5.myworkdayjobs.com/en-US/External/job/Austin/Data-Scientist_R-1",
  );
  assert.equal(job.title, "Data Scientist");
  assert.equal(job.company, "Block");
  assert.equal(job.location, "Austin, TX");
  assert.equal(job.salary, "$130K-$150K");
});

test("LinkedIn job view reads the open top card", () => {
  const job = page("linkedin-job.html", "https://www.linkedin.com/jobs/view/1234567890");
  assert.equal(job.title, "ML Engineer");
  assert.equal(job.company, "OpenAI");
  assert.equal(job.location, "San Francisco, CA");
  assert.equal(job.salary, "$200K–$280K");
});

test("a page that is not a posting yields an empty job", () => {
  const { document } = parseHTML("<html><body><h1>About</h1></body></html>");
  const job = extractJob(document, "https://example.com/about");
  assert.equal(job.title, "");
  assert.equal(job.company, "");
});
