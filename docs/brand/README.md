# Hire Pursuit

Hire Pursuit is a self-hosted job search tool for one person: agents prepare applications, you approve them, and you land the role. The name is a nod to "higher pursuits" with a wink at a certain board game; the identity borrows the game's idea of collecting pieces, never its look.

**Names.** The product is **Hire Pursuit**. The repository (`adrianmross/pursuit`), the binary and every CLI command stay `pursuit`, and commands are always written that way: `pursuit approve APP_ID`.

## The mark and wordmark

The mark is a lowercase **p** built as a game piece: **six pieces**. Five segments make the ring; the **tail is the sixth and last piece**. The tail starts at the middle of the ring, never above it, and the ring opens around it so nothing overlaps. When the logo appears whole, the tail is gold: the last piece is the offer.

The wordmark `hire pursuit` is drawn in the same geometry: a monoline stroke (11 units on a 100-unit body), circular bowls, straight descenders and flat, square-cut ends. Only the p is segmented. It is drawn, not typeset; never retype it in a font.

- `hire-pursuit-wordmark` on light grounds, `hire-pursuit-wordmark-reverse` on dark or brand-forest grounds, `hire-pursuit-wordmark-mono` (all ink) for print and anywhere gold can't go.
- The mark alone where the name is already visible (the dashboard sidebar, a favicon).
- The app icon centers the ring in a brand-forest square and lets the gold tail run off the bottom edge. It reads down to 20px.
- The tail is always brand-gold (#E8C547) in the full-color logo. Don't recolor it or add another gold element beside the logo.
- Clear space: one ring width around the logo. Below 96px wide, use the mark instead of the wordmark.
- Lowercase in the logo, title case in sentences: "Hire Pursuit".

## Voice

The brand says "job search" through its words, not through pictures: no briefcases, ladders or handshakes. It can be wry about the process, never about the person.

- Tagline: **For your higher pursuits.** Secondary line: **Land the role.**
- Buttons are verbs about the search: "Approve", "Reject", "Re-check posting", "Add role".
- Status labels match the API, in plain words: To apply, Preparing, Needs input, Failed, Prepared (`awaiting_review`), Approved, Submitting, Submitted, Applied, Interviewing, Withdrawn. **Offer** is proposed and not yet an API status.
- Agents prepare, the person decides, and copy always says so: "Review before it's sent." Approving never claims to submit.
- Celebrate offers plainly ("Offer from Halcyon. Nice.") and never celebrate volume.

## Visual foundations

**The dashboard is monochrome.** `surface-page`, `surface-card`, `surface-sunken`, `line`, `ink` and `ink-muted` carry the interface; the primary button is `action` (ink).

**Gold means a win.** `brand-gold` appears in the product only as the logo's tail, the Offer chip and the sixth piece of the progress mark. Gold text on light grounds uses `gold-ink`; text on gold is `ink`.

**Forest is for the brand, not the UI.** `brand-forest` and `brand-cream` are for the app icon, marketing and the sign-in panel.

**Progress is pieces.** Each application carries a progress mark: to_apply 1, Prepared 2, Approved 3, Submitted 4, Interviewing 5, Offer 6 (the gold tail). Status chips are told apart by lightness and outline, not hue: waiting-on-you statuses are outlined, Approved is ink, Failed uses `danger` text, Withdrawn is a muted outline, Offer is the only gold chip. `danger` is for errors and destructive actions, never for a rejection.

**Type.** Outfit for headings (`display`, `title`, `heading`), whose round bowls match the mark; Inter for interface text (`body`, `small`, `label`); JetBrains Mono for commands, IDs and scores (`code`).

**Shape.** Cards: `radius-md` on `surface-card` with a `line` border, no shadow. Chips: `radius-pill`. 4px spacing base; cards pad `space-4`, sit `space-6` apart, pages have `space-8` gutters. Every control shows a 2px `focus` ring.

## Components

The dashboard is a React + Tailwind + shadcn app (`web/`), not plain HTML/CSS/JS — this system's `bundle.css`/`bundle.js` describe the intended look, not the dashboard's actual stack; treat them as a spec to port into React components (`web/src/components/`), not files to load as-is. Tokens live as CSS variables in `web/src/index.css`.

Button, StatusChip, ProgressMark, ApplicationRow, ApprovalCard, LinkHealth and CommandBlock each have guidelines and a live preview in this system; their React equivalents live under `web/src/components/`.

## Iconography

Line icons at 1.5px stroke on a 24px grid, square-cut ends to match the wordmark, in `ink` or `ink-muted`. No icon set is part of this system yet.
