# pursuit docs

API and CLI reference for `pursuit`. Next.js + [Nextra](https://nextra.site) for narrative docs, [Scalar](https://github.com/scalar/scalar) for the live OpenAPI reference.

```bash
npm install
npm run dev
```

The site is a static export (`basePath: "/docs"`) that the daemon embeds and serves at `/docs`, same origin as the dashboard and API, so every cross-link is root-relative and moving to a real domain needs no docs changes. The API reference is the daemon's own `/reference` page. From the repo root, `make web` builds the dashboard and these docs into `internal/dashboard/dist`.

Narrative content lives in `content/*.mdx`, ordered by `content/_meta.js`. Pre-release (`0.x`) — see [`content/changelog.mdx`](content/changelog.mdx) for how versions are cut.

```bash
npm run build
```
