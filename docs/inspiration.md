# What the extension borrows

Looked at Simplify Copilot, Teal's Job Search Chrome extension, and Huntr's clipper. All three do the same three jobs: save the posting you already have open, fill the repetitive fields on the ATS form, and keep a status next to that page. Pursuit takes those and leaves the rest.

## Taken

- Pair the browser with the tracker you already run. Simplify and Huntr sign into an account; Pursuit stores a server URL and an API token in extension storage. The token only needs `write` (which includes `read`). It does not need `approve`.
- Save from the posting you are viewing. Teal and Huntr read the open page into the tracker. Pursuit does that, then dedupes on the posting URL so a second save is the same application.
- Read structured data first. Job pages often include schema.org `JobPosting` JSON-LD (title, company, location, salary). Site-specific reads for Greenhouse, Ashby, Lever, Workday, and a LinkedIn job view cover pages that omit it.
- Autofill contact fields and answers you already wrote. Simplify and Huntr fill from a profile; unique answers get reused when the same question shows up. Pursuit fills standing answers from `GET /v1/profile`, then prepared per-application answers when the form matches a saved posting.
- Leave the fields you could not fill obvious, and do not submit. Simplify's own settings say the person reviews the form and clicks Submit. Pursuit outlines missed required fields and never clicks Submit.
- Show whether this page is already tracked, and let the status change from the popup. Huntr asks which stage to save into. Pursuit shows the current status and a dropdown that writes through the existing application update.

## Left out

- Job boards, match scores, keyword extraction, and "open 40 boards at once" search. Teal's Super Search is a different product.
- AI-written answers, résumé tailoring, and cover-letter generation. If Pursuit already prepared an answer, the extension fills it. It does not invent one.
- Autofill on Workday, iCIMS, Taleo, Indeed, or Glassdoor. Capture knows Workday and LinkedIn job views. Filling is Greenhouse, Ashby, and Lever.
- Crawling, background fetches of job pages, and any LinkedIn API or automated LinkedIn request. The content script only reads the document the user opened, and only when the popup asks.
- Clicking Apply or Submit, multi-page "keep filling as you go" bots, and setting a résumé file input. Browsers block that assignment, so the popup downloads the prepared PDF instead.
- Kanban, contacts, tasks, maps, email templates, and unemployment logs. The web dashboard is a separate page served by `pursuit serve`, not part of the extension.
- A company database or prefilled company research. The company name comes from the page.
