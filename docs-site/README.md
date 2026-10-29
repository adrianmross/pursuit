# pursuit docs

API and CLI reference for `pursuit`. Next.js + [Nextra](https://nextra.site) for narrative docs, [Scalar](https://github.com/scalar/scalar) for the live OpenAPI reference.

```bash
npm install
npm run dev
```

The API reference page (`/api-reference`) renders `GET /openapi.json` from a running `pursuit serve` — point it at a non-default server with `NEXT_PUBLIC_PURSUIT_SERVER`.

Narrative content lives in `content/*.mdx`, ordered by `content/_meta.js`. Pre-release: there is no versioned deploy yet — see [`content/changelog.mdx`](content/changelog.mdx) for how changes will be tracked once there is one.

```bash
npm run build
```
