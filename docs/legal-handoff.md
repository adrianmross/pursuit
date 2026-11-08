# Legal handoff

Briefs to give Claude (a fresh session, Claude Docs, or the app) to review and tighten `web-app/content/legal/privacy.md` and `web-app/content/legal/terms.md`. The posture is generic and very permissive: short plain-English terms, no restrictions beyond what the law and basic safety need, and nothing that overreaches on user data. Claude is not a lawyer. Have a real one look before relying on these for anything with money or regulated data in it.

## Fill these in first

| Placeholder | Where | What it is |
|---|---|---|
| Contact email | both pages | Filled: `privacy@hirepursuit.app`, a Porkbun forward to the operator's inbox |
| Jurisdiction | terms | Filled: California law, California state and federal courts, with the user's home courts preserved |
| Operator | both | Whether this is you as an individual or an entity; the pages say "we" |
| Effective date | both | Currently November 7, 2026 |

## Fact sheet (paste into every brief)

**Product.** Pursuit is an open-source (MIT) job-application tracker: a Go server with a web dashboard, a CLI (`pursuit`), a Chrome/Firefox extension, and an HTTP API. Anyone can self-host. The operator also runs a hosted multi-tenant service at hirepursuit.app.

**Operator.** Adrian Ross, an individual (copyright holder in `LICENSE`). No company entity yet.

**Hosted stack and processors.** Vercel (hosting, Go function). Neon (Postgres, plus Neon Auth / Better Auth for email and password, 6-digit email codes, optional social or OIDC sign-in, sent from Neon's shared sender). Optional S3-compatible object storage for uploaded documents. Passkeys are Pursuit's own WebAuthn, stored as public keys. PostHog receives dashboard page views and two named events (signed_up, application_added), cookieless, no replay, no query strings, anonymous. Sentry also receives dashboard browser errors. Sentry receives server error reports (route, status, request ID, stack trace; no headers, cookies, query strings, bodies, or account details). Cloudflare Web Analytics (cookieless page views and Core Web Vitals) is on for hirepursuit.app. No ad or tracking tools. Billing is not live.

**Self-hosted telemetry.** `pursuit serve` sends an anonymous ping at start and daily to a Cloudflare Worker run by the operator (random install ID, version, OS, architecture, database type, container flag; no user content; the Worker neither reads nor stores IP addresses). It is on by default and off with `PURSUIT_TELEMETRY=off` or `DO_NOT_TRACK=1`. A persistent random install ID may count as personal data under GDPR, so the question for review is whether opt-out is acceptable or consent is needed for EU installs.

**Data held for a hosted user.** Email, display name, avatar. Applications, companies, roles, notes, interview rounds, tasks, custom fields. A profile of standing answers (name, contact, work authorization, free-text answers). Uploaded résumés and cover letters. Hashed access tokens with scopes. An audit log of changes with actor, token, and timestamps. Request logs (IP, user agent, timestamps) at the hosting providers. Demo sandbox: throwaway accounts, IP address used only for rate limiting.

**Sensitive-ish content.** Résumés and application answers can contain employment history, contact details, and work-authorization or visa status. There is no payment data, no health data, and no government ID collection in the product.

**Outbound behavior.** A daily link checker fetches job-posting URLs that users saved, to see whether they are still open. It sends the URL, not personal data. Agents and the CLI act through user-issued tokens; an agent can prepare an application but approving requires a separate `approve` scope, and Pursuit never presses Submit on a form itself.

**Browser extension.** Stores server URL and token in `chrome.storage.local`. Content script runs on all http(s) pages and frames to detect job postings and application forms, then reads the posting and form fields. Talks only to the user's paired server. No remote code, no analytics. It is distributed through the Chrome Web Store, so the privacy policy has to satisfy the Chrome Web Store User Data Policy (single purpose, accurate disclosure, no sale or unrelated use, no advertising use).

**Cookies and storage.** `pursuit_session` cookie, 30 days. Theme and interface preferences in local storage.

**Deletion and export.** Pipeline export as JSON (Settings, or `pursuit export json`). Deletion is by request to the contact today; there is no self-serve account delete.

**Open PRs that change these facts when they merge.** #102 (Stripe: a payment processor, billing goes live), #103 (S3 documents), and #105 (smart search, which may add an embedding provider, so "no model training" needs re-checking). Re-run Brief C when any of them lands.

**Audience.** Adults looking for work. Not directed at children under 16.

## Posture (paste into every brief)

- Generic and very permissive. Plain English, short, no legalese for its own sake.
- Users own their data and can leave with it. No claim over content beyond running the service.
- No selling data, no ad use, no model training on user content.
- Minimal restrictions on use: lawful use, no attacks or abuse, user is responsible for what they submit to employers and for the tokens they issue.
- Standard protections for the operator are fine and expected: as-is disclaimer, limited liability, right to suspend for abuse, right to change the service. Keep them proportionate, not aggressive.
- Works for the self-hosted case too: the terms say the MIT License governs self-hosted installs.
- Wherever a stronger local privacy law (GDPR, UK GDPR, CCPA/CPRA) would apply, honor it without building a separate regime.

## Brief A: Privacy policy

> You are acting as a careful commercial and privacy lawyer reviewing a draft privacy policy for a small open-source product. You are not giving legal advice and I understand that; produce the best draft a lawyer would approve.
>
> Context: [paste Fact sheet and Posture]
>
> The draft is `web-app/content/legal/privacy.md` [paste it]. Review it for:
> 1. Anything it says that the fact sheet doesn't support, or that the fact sheet shows is false. Fix or remove it.
> 2. Anything the fact sheet shows that it omits and a privacy law or the Chrome Web Store User Data Policy would expect it to disclose.
> 3. GDPR / UK GDPR basics (controller identity, purposes, lawful bases in one plain sentence each, processors, transfers, retention, rights, complaint to a regulator) and CCPA/CPRA basics (categories collected, no sale or sharing, rights, no discrimination), kept to the shortest honest wording. Don't add a separate section per regime unless the law requires it.
> 4. Whether "we don't use it to train models" and "we'll delete on request" are commitments the operator can actually keep, given the fact sheet.
> 5. Tone: keep it plain and permissive. Cut anything defensive that isn't needed.
>
> Return: (a) the revised MDX, ready to commit; (b) a short list of changes and why; (c) the questions you need me to answer that change the text; (d) anything you'd want a human lawyer to check. Keep placeholders in square brackets.

## Brief B: Terms of Service

> You are acting as a careful commercial lawyer reviewing draft terms of service for a small open-source product with a hosted service. You are not giving legal advice and I understand that; produce the best draft a lawyer would approve.
>
> Context: [paste Fact sheet and Posture]
>
> The draft is `web-app/content/legal/terms.md` [paste it]. Review it for:
> 1. Enforceability of the disclaimer and limitation of liability as written, and the narrowest wording that still protects an individual operator. Flag what varies by jurisdiction.
> 2. Whether "you are responsible for what you submit" and the agent/token language puts the right responsibility on the user without making the operator the guarantor of employer terms.
> 3. Whether the content license to the operator is as narrow as it can be and still let the service run, back up, and display content.
> 4. Whether the self-hosted carve-out (MIT governs) is clean, and whether the hosted terms conflict with MIT.
> 5. Missing basics for a hosted service: account termination, notice of changes, shutdown with export, DMCA or takedown handling if uploaded documents could host third-party content, governing law and venue, severability. Add only what is needed.
> 6. Consumer-protection issues if users pay later (billing is not live): what would need to change, listed, not drafted.
> 7. Tone: plain and permissive. Cut anything that is only there to look serious.
>
> Return: (a) the revised MDX, ready to commit; (b) a short list of changes and why; (c) the questions you need me to answer that change the text; (d) anything you'd want a human lawyer to check. Keep placeholders in square brackets.

## Brief C: Cross-check before publishing

> Here are the final privacy policy and terms of service for the same product [paste both] and the fact sheet [paste]. Check that they agree with each other (definitions, who "we" is, contact, deletion and export promises, data categories) and with the fact sheet. List every inconsistency, every promise the product doesn't keep today, and every placeholder still unfilled. Do not rewrite; just list.

## Using the output

Paste the revised markdown over `web-app/content/legal/privacy.md` and `terms.md`, fill the placeholders, run Brief C, then ship it. The Chrome Web Store listing's privacy policy URL is `/privacy` on the deployed dashboard.
