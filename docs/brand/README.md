# Pursuit

Pursuit is a self-hosted job search tool: you aim at a role, agents help you prepare, you approve, and you land the offer. The identity is built on one idea, **the bullseye**: the role you are aiming at. Everything else stays quiet so that the moment you hit it stands out.

## The mark and wordmark

The mark is a lowercase **p** whose bowl is a target, with a gold center. In the wordmark that same p starts the word, so the logo and the name are one thing: `pursuit` is drawn in the same geometry as the mark, a monoline stroke (11 units on a 100-unit body), circular bowls and flat, square-cut ends. The i takes a round dot that echoes the bullseye. The wordmark is drawn, not typeset; never retype it in a font.

- Use `pursuit-wordmark` on light grounds and `pursuit-wordmark-reverse` on dark or brand-forest grounds.
- Use the mark alone where the name is already visible (the dashboard sidebar, a favicon); use `pursuit-app-icon` for app and repo icons.
- The gold center is always brand-gold (#E8C547). Don't recolor it, don't drop it, don't add a second gold element next to the logo.
- Keep clear space of one bowl width around the logo. The wordmark's minimum width is 80px; below that use the mark.
- The name is always lowercase in the logo and title case in sentences: "Pursuit".

## Speaking about careers

The brand says "job search" through its words more than through pictures: no briefcases, ladders or handshakes. Lead with the outcome and the action.

- Tagline: **Land the role.**
- Headlines name the goal: "Every application, one pipeline." "Your next role, on your terms."
- Buttons are verbs about the search: "Add role", "Approve and send", "Log interview", "Mark offer".
- Status names are plain words the person would say out loud: Saved, Applied, Interviewing, Offer, Closed.
- Celebrate offers plainly and briefly ("Offer from Acme. Nice.") with gold; never celebrate volume ("You applied to 40 jobs!").
- Agents prepare, the person decides. Copy always makes the approval step explicit: "Review before it's sent."

## Visual foundations

**The dashboard is monochrome.** Grays and ink (`surface-page`, `surface-card`, `surface-sunken`, `line`, `ink`, `ink-muted`) carry the whole interface. The primary button is `action` (ink), not a color. This keeps a long list of applications calm to scan.

**Gold means a win.** `brand-gold` appears in the product only for the Offer status, a goal reached, and the bullseye in the logo. On light grounds, gold text uses `gold-ink`; on gold, text is `ink`. If gold shows up anywhere else, it stops meaning anything.

**Forest is for the brand, not the UI.** `brand-forest` and `brand-cream` are for the app icon, the marketing site and the sign-in panel. Inside the dashboard they appear only through the logo.

**Status without color.** Application stages are told apart by label and position, not hue: chips use `surface-sunken` with `ink` text in the `label` style; Offer flips to `brand-gold` with `ink`; Closed uses `ink-muted`. `danger` is for destructive actions and errors, never for a rejection.

**Type.** Headings use Outfit (`display`, `title`, `heading`), a geometric face whose round bowls match the mark. Interface text uses Inter (`body`, `small`, `label`). CLI commands and IDs use JetBrains Mono (`code`), since Pursuit is CLI first.

**Shape.** Cards use `radius-md` on `surface-card` with a `line` border and no shadow. Chips use `radius-pill`. Spacing follows a 4px base (`space-1` to `space-8`); cards pad `space-4`, sit `space-6` apart, and pages have `space-8` gutters.

**Focus.** Every interactive element shows a 2px `focus` ring outside its edge: forest in light, gold in dark.

## Iconography

Line icons at 1.5px stroke on a 24px grid, square-cut ends to match the wordmark, in `ink` or `ink-muted`. No filled icons except the bullseye dot. No icon set is part of this system yet.
