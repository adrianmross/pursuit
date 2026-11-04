# What the extension borrows

Looked at Simplify Copilot, Teal's Job Search Chrome extension, and Huntr's clipper. All three do the same three jobs: save the posting you already have open, fill the repetitive fields on the ATS form, and keep a status next to that page. Pursuit takes those and leaves the rest.

## Taken

- Pair the browser with the tracker you already run. Simplify and Huntr sign into an account; Pursuit stores a server URL and an API token in extension storage. The token only needs `write` (which includes `read`). It does not need `approve`.
- Save from the posting you are viewing. Teal and Huntr read the open page into the tracker. Pursuit does that, then dedupes on the posting URL so a second save is the same application.
- Read structured data first. Job pages often include schema.org `JobPosting` JSON-LD (title, company, location, salary). Site-specific reads for Greenhouse, Ashby, Lever, Workday, and a LinkedIn job view cover pages that omit it.
- Autofill contact fields and answers you already wrote. Simplify and Huntr fill from a profile; unique answers get reused when the same question shows up. Pursuit fills standing answers from `GET /v1/profile`, then prepared per-application answers when the form matches a saved posting.
- Leave the fields you could not fill obvious, and do not submit. Simplify's own settings say the person reviews the form and clicks Submit. Pursuit outlines missed required fields and never clicks Submit.
- Show whether this page is already tracked, and let the status change from the page. Huntr asks which stage to save into. Pursuit shows the current status, the progress mark, and a dropdown that writes through the existing application update.
- Work on the page, not in a popup. Simplify puts a button on the posting and a panel beside the form. Pursuit does the same: a corner button on postings and application forms opens a panel (Alt+Shift+P). The popup is only for pairing.
- Attach the résumé. Simplify drops your résumé into the upload field. Pursuit builds the file through `DataTransfer`, the same path a drag and drop uses: the application's tailored `resume_pdf`, else the newest `base_resume`.
- Learn answers as you go. Simplify offers to save what you typed. Pursuit stacks the questions it could not answer as notes, scrolls to each outlined field in turn, and Save my answer keeps what you typed as a standing answer, so the next form gets them.
- Mark it applied after you submit. Simplify tracks the submit. Pursuit offers Mark applied once the form is sent, and leaves approved roles to the agent holding the submit work.

## Left out

- Job boards, match scores, keyword extraction, and "open 40 boards at once" search. Teal's Super Search is a different product.
- AI-written answers, résumé tailoring, and cover-letter generation. If Pursuit already prepared an answer, the extension fills it. It does not invent one.
- Tuned autofill for Workday, iCIMS, Taleo, Indeed, or Glassdoor. Matching is by label, so simple forms anywhere fill; Workday's multi-step widgets and typeaheads that load options from the network (city search) are left outlined.
- Crawling, background fetches of job pages, and any LinkedIn API or automated LinkedIn request. The content script only reads the document the user opened.
- Clicking Apply or Submit, and multi-page "keep filling as you go" bots. Fill each step with the panel; the person, or the agent holding approved submit work, clicks Submit.
- Kanban, contacts, tasks, maps, email templates, and unemployment logs. The web dashboard is a separate page served by `pursuit serve`, not part of the extension.
- A company database or prefilled company research. The company name comes from the page.
