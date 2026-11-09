# Privacy

Effective November 7, 2026.

Pursuit is a job-application tracker run by Adrian Ross, an individual. This page covers the hosted service at hirepursuit.app, the browser extension, and what changes if you run it yourself. The short version: your applications are yours, we use them only to run the service for you, and we don't sell them, share them for advertising, or track you across the web.

## If you run it yourself

Self-hosted Pursuit (`pursuit serve`) sends us one anonymous ping at start and daily: a random install ID, the version, OS, architecture, database type, and whether it runs in a container. Nothing from your data is in it, and you can turn it off with `PURSUIT_TELEMETRY=off` or `DO_NOT_TRACK=1`; see [Telemetry](/docs/telemetry). Your database, documents, and tokens stay on your machine or your infrastructure, and the rest of this page doesn't apply to them. The rest of this page is about the hosted service.

## What the hosted service holds

- **Your account:** your email, a display name and avatar if you set them, and your sign-in credentials. Passwords are handled by our authentication provider, and tokens are stored hashed. A passkey is stored as a public key.
- **What you put in:** applications, companies, notes, interview rounds, tasks, custom fields, your profile and prepared answers, and the résumés and cover letters you upload.
- **A record of changes:** an audit log of what changed, when, and which token or sign-in did it, so you can see what an agent did on your behalf.
- **Operational data:** request logs (IP address, user agent, timestamps, request IDs) kept by our hosting providers for security and debugging.
- **Error reports:** when the hosted server hits a panic or a 5xx response, it sends the route, status, request ID and stack trace to Sentry. It sends no headers, cookies, query strings, request bodies, or account details.
- **Demo visitors:** the try-it-without-an-account sandbox creates throwaway data. We use your IP address only to rate-limit it.

## What we do with it

We use it to run Pursuit for you: show your pipeline, prepare applications, check whether saved postings are still open, send you a sign-in code, and keep the service secure. We don't sell it, we don't use it for advertising, and we don't use it to train models.

The posting check fetches the job URLs you saved. It sends the URL, not your personal data.

## Who else handles it

We use a few providers to run the service, and they process data on our behalf:

- Vercel, for hosting.
- Sentry, for server and browser error reports.
- PostHog, for a few product events (see Cookies and storage).
- Cloudflare, for cookieless page-view analytics.
- Neon, for the database and for sign-in, including the emails that carry sign-in codes.
- An S3-compatible object store, for uploaded documents, when it's turned on.
- Google or another sign-in provider, only if you choose to sign in that way.

We may add or swap providers. They are bound to use the data only to provide their service to us. We may also disclose data when the law requires it, or to protect the service and its users.

## Cookies and storage

Signing in sets a session cookie (`pursuit_session`) that lasts 30 days. Your theme and a few interface preferences are kept in your browser's local storage. There are no advertising or analytics cookies. The hosted dashboard uses Cloudflare Web Analytics to count page views and measure page speed. It sets no cookies and doesn't track you across sites or follow you between visits.

The dashboard also sends page views and two named events (signing up and adding an application) to PostHog, and reports browser errors to Sentry. Neither stores anything in your browser, uses session replay or records what you type or click, or includes query strings, your account, or anything you enter. PostHog respects your browser's Do Not Track setting. Self-hosted dashboards send none of this.

## The browser extension

- It stores the server URL and access token you pair it with, in the browser's extension storage on your device.
- On a page that looks like a job posting or application form, it reads the posting and the form's fields so it can save the job and fill the form. It doesn't crawl or read other pages.
- It sends the posting you choose to save, and requests for your profile, prepared answers, and résumé, to the server you paired, and nowhere else.
- It doesn't send data to us, to analytics, or to advertisers, and it never presses Submit.

It runs on every site because job postings and forms live on company sites and on boards such as Greenhouse, Lever, and Ashby, often inside embedded frames. The check for whether a page is one runs on your device.

## Keeping and deleting your data

We keep your data while your account is open. You can export your pipeline at any time from Settings in the dashboard or with `pursuit export json`. To delete your account and its data, ask us at the contact below and we'll do it, except for copies in backups, which expire on their normal schedule, and anything we must keep by law.

## Your rights

Wherever you live, you can ask to see, correct, export, or delete your data, and to object to how it's used. If you're in the EEA, UK, California, or elsewhere with a privacy law that says more, those rights apply and we'll honor them. We won't treat you differently for using them. Ask at the contact below.

We process your data to provide the service you asked for (contract), and for security and legal compliance (legitimate interests). Our providers are in the United States, so if you're outside it, your data is transferred there under their standard contractual protections. If you're in the EEA or UK, you can also complain to your local data protection authority.

## Security

We protect your data with standard measures: encrypted connections, hashed tokens, and scoped access tokens. No system is perfectly secure, and we can't promise otherwise. If something affecting you goes wrong, we'll tell you.

## Children

Pursuit is for people old enough to work. It isn't directed at children under 16, and we don't knowingly collect their data.

## Changes

If we change this page in a way that matters, we'll update the date above and, for hosted accounts, tell you. Keep using Pursuit after a change and you accept it. Older versions stay in the repository history.

## Contact

Write to privacy@hirepursuit.app. Use email for anything that includes personal data. For other questions you can open an issue at [github.com/adrianmross/pursuit](https://github.com/adrianmross/pursuit/issues), but issues are public.
