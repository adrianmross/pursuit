# Changelog

## [0.3.0](https://github.com/adrianmross/pursuit/compare/v0.2.1...v0.3.0) (2026-10-31)


### Features

* **pursuit:** Add passkey sign-in ([#88](https://github.com/adrianmross/pursuit/issues/88)) ([127280e](https://github.com/adrianmross/pursuit/commit/127280ec5bbf7a2353688d74953f75c919e260aa))
* **pursuit:** Add password reset to the dashboard ([#70](https://github.com/adrianmross/pursuit/issues/70)) ([ca1dad5](https://github.com/adrianmross/pursuit/commit/ca1dad5bc41d9e70f5b28dffe03fafb128fa7fc7))
* **pursuit:** Add PURSUIT_SIGNUPS=closed to pause new accounts while existing users keep signing in ([#69](https://github.com/adrianmross/pursuit/issues/69)) ([dc7b865](https://github.com/adrianmross/pursuit/commit/dc7b865122f4004e86612c40208f16d97d0a0438))
* **pursuit:** Cache dashboard assets on the CDN and add Cloudflare Web Analytics ([#66](https://github.com/adrianmross/pursuit/issues/66)) ([0a023e8](https://github.com/adrianmross/pursuit/commit/0a023e8552733befacda355b5b58a172de550143))
* **pursuit:** Edit every application field in place, company included, with silent background saves ([#84](https://github.com/adrianmross/pursuit/issues/84)) ([4f80a08](https://github.com/adrianmross/pursuit/commit/4f80a08f0c91ff4800942789924516d4e1c0e65e))
* **pursuit:** Hide row selection behind a Select toggle, collapse table controls on mobile and soften dark-mode and failure colors ([#82](https://github.com/adrianmross/pursuit/issues/82)) ([d5d8d82](https://github.com/adrianmross/pursuit/commit/d5d8d82935cff0fa6441fd1552725c06708cb2be))
* **pursuit:** Keep the dashboard session in an httpOnly cookie, refuse cross-site writes, and honor proxy auth only on Vercel previews ([#64](https://github.com/adrianmross/pursuit/issues/64)) ([65080ea](https://github.com/adrianmross/pursuit/commit/65080ea22830bfccc46ae931ffe9dc207e98f5e8))
* **pursuit:** Move standing questions to their own searchable page and make them easier to add and edit ([#80](https://github.com/adrianmross/pursuit/issues/80)) ([11700f9](https://github.com/adrianmross/pursuit/commit/11700f91f299b51af7ab28457d061d765c4100dd))
* **pursuit:** rebuild account settings with a change-password dialog ([#86](https://github.com/adrianmross/pursuit/issues/86)) ([bc35541](https://github.com/adrianmross/pursuit/commit/bc355412f9e93718d138c664874826965c6160c2))
* **web-app:** add a theme toggle to the public pages ([#87](https://github.com/adrianmross/pursuit/issues/87)) ([875d7f2](https://github.com/adrianmross/pursuit/commit/875d7f29de43e3b03ac381c852ceb4ffee3f0eb3))
* **web-app:** add back navigation, move the command palette to Cmd+P and use Cmd+K for search ([#81](https://github.com/adrianmross/pursuit/issues/81)) ([117e72a](https://github.com/adrianmross/pursuit/commit/117e72adbb7dc1431c2b2603869ccc61bd33e77c))


### Bug Fixes

* **cli:** Reach a deployment behind Vercel protection with a bypass secret ([#68](https://github.com/adrianmross/pursuit/issues/68)) ([0a23635](https://github.com/adrianmross/pursuit/commit/0a23635a7ae56f5b02f983130586ace0b50ed74c))
* **pursuit:** Add a hot-reloading pursuit-preview command for worktrees ([#85](https://github.com/adrianmross/pursuit/issues/85)) ([7f879d2](https://github.com/adrianmross/pursuit/commit/7f879d228d7d74869236e1956efe480a390c4ff0))
* **pursuit:** Deploy the hosted app to production only from release tags, not every merge to main ([#79](https://github.com/adrianmross/pursuit/issues/79)) ([c5e5409](https://github.com/adrianmross/pursuit/commit/c5e5409e1c6e42d1f84f68e955f46055f0c89b3c))
* **pursuit:** fix the API reference page, a command palette crash and stalls between the home page and sign-in ([#83](https://github.com/adrianmross/pursuit/issues/83)) ([a0ec57f](https://github.com/adrianmross/pursuit/commit/a0ec57f1ab26ceff0ed356d9644093fbda1d5501))
* **pursuit:** Keep dashboard sign-ins alive instead of expiring with the 15-minute Neon Auth token ([#77](https://github.com/adrianmross/pursuit/issues/77)) ([1f4c067](https://github.com/adrianmross/pursuit/commit/1f4c0673b3ce3ce18bf9f4841e682785cb9e3f8a))
* **pursuit:** Keep production data out of preview databases with Neon masking rules ([#72](https://github.com/adrianmross/pursuit/issues/72)) ([47771f0](https://github.com/adrianmross/pursuit/commit/47771f0ac79bb9fe1067a611439c9c42a4322759))
* **pursuit:** serve one llms.txt and make the root copy a pointer to it ([#74](https://github.com/adrianmross/pursuit/issues/74)) ([b5e0e8d](https://github.com/adrianmross/pursuit/commit/b5e0e8deca9bb370f3357191fb4e8646663428e2))
* **pursuit:** Show where a device login came from and explain why some sessions cannot approve ([#75](https://github.com/adrianmross/pursuit/issues/75)) ([29b7a4c](https://github.com/adrianmross/pursuit/commit/29b7a4ca87684d26a5a4bc9bdddd0fdda8d110e4))
* **pursuit:** Simplify the home page hero and turn Get started into a wordmark link ([#78](https://github.com/adrianmross/pursuit/issues/78)) ([d4a1894](https://github.com/adrianmross/pursuit/commit/d4a1894bfea7f900bdf2ede3ddf32d20e271ca25))

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
