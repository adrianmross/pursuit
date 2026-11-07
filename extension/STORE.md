# Chrome Web Store listing

Paste these into the Developer Dashboard. Upload `pursuit-extension_X.Y.Z.zip` from the GitHub Release.

**Name:** Pursuit

**Summary (132 characters max):** Save the job you're viewing and fill the application from your Pursuit tracker. Never presses Submit.

**Category:** Productivity

**Description:**
Pursuit keeps track of every role you're chasing. This extension adds two things to the pages where you apply:

- Save the posting you have open to your tracker, with undo.
- Fill the application form from your profile and prepared answers, one field at a time, and attach your résumé. Questions it can't answer are outlined; type the answer once and it is reused on the next form.

It never presses Submit, and it talks only to the Pursuit server you pair it with. Pair it from the popup with your server URL and a token that can write but not approve.

Requires a Pursuit server (self-hosted or hosted). The dashboard has a practice application at /practice.html for trying it.

## Privacy tab

**Single purpose:** Save job postings to your Pursuit tracker and fill application forms from your profile.

**Permission justifications**

- `storage`: Keeps the Pursuit server URL and access token you enter when pairing, on your device.
- `activeTab`: Opens the panel on the tab you choose, from the toolbar button or Alt+Shift+P.
- Content script on all http(s) pages (the "host permission" prompt): Job postings and application forms live on company sites and on boards like Greenhouse, Lever, and Ashby, often inside embedded frames. The script only checks whether the page is a posting or a form, on your device, and loads the panel there if it is.
- Remote code: none. All code ships in the package.

**Data usage disclosures:** check "Personally identifiable information" and "Website content". Certify that data is not sold, not used for unrelated purposes, and not used for creditworthiness or lending.

**Privacy policy URL:** `<your dashboard origin>/docs/privacy`

## Before you submit

- Screenshots: at least one at 1280×800. The practice application (`/practice.html`) with the panel open, and the filled state, make good ones.
- The server must allow the extension's origin. After the first upload the store assigns the extension ID; add `chrome-extension://<id>` to `PURSUIT_CORS_ORIGINS` on any server it should reach.
- The manifest's `gecko` id and `version` are release-managed; don't edit them for the store.
