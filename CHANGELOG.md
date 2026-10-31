# Changelog

## [0.2.1](https://github.com/adrianmross/pursuit/compare/v0.2.0...v0.2.1) (2026-10-31)


### Bug Fixes

* **pursuit:** Deploy Vercel previews only for pull requests labeled preview ([#62](https://github.com/adrianmross/pursuit/issues/62)) ([f70b43d](https://github.com/adrianmross/pursuit/commit/f70b43db6c1fa87111d3a41730ae4713687e8305))

## [0.2.0](https://github.com/adrianmross/pursuit/compare/v0.1.0...v0.2.0) (2026-10-31)


### Features

* **pursuit:** Add a Connect an agent page with a setup prompt, skill and token ([#59](https://github.com/adrianmross/pursuit/issues/59)) ([d2ddaaa](https://github.com/adrianmross/pursuit/commit/d2ddaaa45540f9dcda09f3745ca4372e031e3b93))
* **pursuit:** Add a profile page, user documents with previews, and owner identity with avatars ([84a0a1c](https://github.com/adrianmross/pursuit/commit/84a0a1cf0316e74d93390acbf37629b19dd49b14))
* **pursuit:** Add a status chooser, rounds as pies, and tasks on every stage ([#58](https://github.com/adrianmross/pursuit/issues/58)) ([db9037c](https://github.com/adrianmross/pursuit/commit/db9037c0e1587b7c786d84ba344b6c5ed047b73e))
* **pursuit:** Add pursuit auth login with browser and device OAuth2 flows ([#48](https://github.com/adrianmross/pursuit/issues/48)) ([4502fb6](https://github.com/adrianmross/pursuit/commit/4502fb6c1547f7c2ba98cd93b828161d13b6b9df))
* **pursuit:** Let the owner behind Vercel deployment protection change their email ([#57](https://github.com/adrianmross/pursuit/issues/57)) ([2720c4d](https://github.com/adrianmross/pursuit/commit/2720c4dfba6f937c35551859981cefa93196a28b))


### Bug Fixes

* **pursuit:** generate api/openapi.json from the served spec and fail CI on drift ([#52](https://github.com/adrianmross/pursuit/issues/52)) ([a90bd16](https://github.com/adrianmross/pursuit/commit/a90bd162a1adaad02dc78fbaad8d316d21430854))
* **pursuit:** Restore the shadcn styling on the pipeline, sign-in and audit search ([#55](https://github.com/adrianmross/pursuit/issues/55)) ([3d2ea09](https://github.com/adrianmross/pursuit/commit/3d2ea0916d7ee6fcebce87a739228c56175a7ff7))
* **pursuit:** Test migrations and the store against Postgres in CI ([#56](https://github.com/adrianmross/pursuit/issues/56)) ([b89be44](https://github.com/adrianmross/pursuit/commit/b89be447369e0724aca75b1a08f4057843fd2813))
* **pursuit:** Tokens can only mint scopes they already hold ([#54](https://github.com/adrianmross/pursuit/issues/54)) ([f01328c](https://github.com/adrianmross/pursuit/commit/f01328ced149016714b63488c3482ac34aa77c3d))

## 0.1.0 (2026-10-31)


### Features

* **api:** POST /v1/batch runs up to 100 requests as the caller ([a9717aa](https://github.com/adrianmross/pursuit/commit/a9717aa964e324b9a955ed3d985892b186c23ff0))
* audit search and filters, search syntax and autocomplete, keyboard table navigation, quick actions ([db649dd](https://github.com/adrianmross/pursuit/commit/db649ddc9c9cccd57f9cf91d21aea9cbe185cea2))
* audit search and time filters, search syntax settings page, responsive search bar ([217824d](https://github.com/adrianmross/pursuit/commit/217824dc01a8b74764ef2453f6962260adba1e69))
* batch API requests and --all-pages for agents ([9009017](https://github.com/adrianmross/pursuit/commit/9009017005e0f87e3672628fa323ff19728294d5))
* **cli:** pursuit batch and a global --all-pages flag ([6ae94cb](https://github.com/adrianmross/pursuit/commit/6ae94cb3a8329f6d91beaec0fab24cd3cb21f99c))
* **dashboard:** themed 404 page served with a 404 status ([308bfaf](https://github.com/adrianmross/pursuit/commit/308bfafd5351ff5880f28054e6999c8ae312ebef))
* **docs:** document batch requests and --all-pages for agents ([43a842b](https://github.com/adrianmross/pursuit/commit/43a842b9ad095a1664de0fb72d1bc1ed35f68f8a))
* **favicon:** validate fetched icons, per-company icon_url, optional Google fallback ([9faa39a](https://github.com/adrianmross/pursuit/commit/9faa39a861a19413321f89e247f05331e2d81f48))
* **favicon:** validate fetched icons, per-company icon_url, optional Google fallback ([d7567bb](https://github.com/adrianmross/pursuit/commit/d7567bbfb71a1df28f4d853e638236ad4fa79326))
* **footer:** link the brand to / and light the heart on hover ([137f9f8](https://github.com/adrianmross/pursuit/commit/137f9f876efca744f8c8572e283db31995cb7d2c))
* **pursuit:** Version the CLI, API, docs and extension together and publish pre-releases ([#44](https://github.com/adrianmross/pursuit/issues/44)) ([624f194](https://github.com/adrianmross/pursuit/commit/624f194009a170c54557fe722a1b1d981c6d6071))
* **web-app:** /start and /applications routes, themed error states ([17dc605](https://github.com/adrianmross/pursuit/commit/17dc605be15387a5f2c78dee74107df2005978af))
* **web-app:** hand-drawn crash and failed sign-in states ([9799112](https://github.com/adrianmross/pursuit/commit/9799112826a358c3e963320464ed883695850834))
* **web-app:** signed-in home at /start, pipeline at /applications ([566e363](https://github.com/adrianmross/pursuit/commit/566e363278caa1244fcd7118c946952b2bbd299a))


### Bug Fixes

* **docs:** green app icon in the welcome hero, uniform step cards ([59fa461](https://github.com/adrianmross/pursuit/commit/59fa46176da74ddc28ed7dd3f7abaf6e4e576efd))
* **docs:** green app icon in the welcome hero, uniform step cards ([994dfcc](https://github.com/adrianmross/pursuit/commit/994dfccfde61fede53c9ac952742cc07a8c7148d))
