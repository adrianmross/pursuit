# pursuit docs

API and CLI reference for `pursuit`. Next.js + [Nextra](https://nextra.site) for narrative docs, [Scalar](https://github.com/scalar/scalar) for the live OpenAPI reference.

```bash
npm install
npm run dev
```

The site is built with `basePath: "/docs"`, so it is meant to be served at `/docs` on the same origin as the dashboard and API; links to `/` and `/reference` are root-relative. If you deploy it on its own origin, set `NEXT_PUBLIC_SITE_URL` (e.g. `https://hire-pursuit.app`) at build time. Moving to a real domain is that one variable plus a `/docs` rewrite to this project; no content changes. The API reference itself is the daemon's own `/reference` page.

Narrative content lives in `content/*.mdx`, ordered by `content/_meta.js`. Pre-release: there is no versioned deploy yet — see [`content/changelog.mdx`](content/changelog.mdx) for how changes will be tracked once there is one.

```bash
npm run build
```
