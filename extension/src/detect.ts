// Is this page a job posting or an application form? Shared by the content
// script (which decides whether to load the panel) and the panel itself.

import { extractJob } from "./extract";

const ATS = /greenhouse\.io|ashbyhq\.com|lever\.co|myworkdayjobs\.com|smartrecruiters\.com|icims\.com|workable\.com/;

// An application form: an email field beside a file upload, or on a known ATS host.
export function hasForm(): boolean {
  const email = document.querySelector("input[type=email], input[name*=email i], input[id*=email i]");
  if (!email) return false;
  return Boolean(document.querySelector('input[type="file"]')) || ATS.test(location.hostname);
}

export function relevant(): boolean {
  return hasForm() || Boolean(extractJob(document, location.href).title);
}
