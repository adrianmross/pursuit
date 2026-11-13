// Tells appkit-ui what this product is: its name, its own preferences and
// its keyboard shortcuts. Imported for its effect by the root layout and by
// the client providers, so it runs on the server and in the browser before
// anything renders.
import { configure, registerBindings, registerPrefs } from "@adrianmross/appkit-ui"
import { SiteFooter } from "@/components/SiteFooter"
import { SnakeEyes } from "@/components/Sketch"
import { BRAND } from "@/lib/brand"

configure({ ...BRAND, footer: <SiteFooter />, failArt: <SnakeEyes className="w-full" /> })

registerPrefs({
  // ponytail: density, accent and stripes are read by nothing; they go when the settings dialog comes from appkit-ui.
  density: "comfortable", accent: "neutral", stripes: "off",
  roundCounter: "wheel", faviconService: "default", funnel: "off",
  // The application page's cards ("on" or "off"), and where each outline tab
  // shows: "tab" inside Details, "card" as its own card, or "off".
  cardListing: "on", cardRelated: "on", cardArtifacts: "on", cardHistory: "on",
  tabRounds: "tab", tabTasks: "tab", tabQuestions: "tab", tabNotes: "tab",
  // The start page's feed rows, "off" once dismissed.
  hintProfile: "on", hintAgent: "on", hintTerminal: "on", hintExtension: "on",
})

registerBindings([
  { id: "searchApps", label: "Search applications", group: "General", combo: "mod+k", when: "app" },
  { id: "home", label: "Go to start", group: "Navigation", combo: "g h", when: "app" },
  { id: "pipeline", label: "Go to applications", group: "Navigation", combo: "g p", when: "app" },
  { id: "archived", label: "Go to archived", group: "Navigation", combo: "g a", when: "app" },
  { id: "back", label: "Back to all applications", group: "Navigation", combo: "Escape", when: "app" },
  { id: "new", label: "Add application", group: "Navigation", combo: "n", when: "app" },
  { id: "rowNext", label: "Next application in the table", group: "Table", combo: "j|ArrowDown|ctrl+n", when: "app" },
  { id: "rowPrev", label: "Previous application in the table", group: "Table", combo: "k|ArrowUp|ctrl+p", when: "app" },
  { id: "rowOpen", label: "Open the highlighted application", group: "Table", combo: "Enter", when: "app" },
  { id: "rowMenu", label: "Quick actions for the highlighted application (then S status, A archive, D delete)", group: "Table", combo: ".|m", when: "app" },
  { id: "rowStatusNext", label: "Move the highlighted application to the next status", group: "Table", combo: "]", when: "app" },
  { id: "rowStatusPrev", label: "Move the highlighted application to the previous status", group: "Table", combo: "[", when: "app" },
  { id: "rowSelect", label: "Select the highlighted application", group: "Table", combo: "x|Space", when: "app" },
  { id: "rowClear", label: "Clear the selection and highlight", group: "Table", combo: "Escape", when: "app" },
  { id: "appStatus", label: "Change the status", group: "Application", combo: "s", when: "app" },
  { id: "appStageNext", label: "Move to the next stage", group: "Application", combo: "]", when: "app" },
  { id: "appStagePrev", label: "Move to the previous stage", group: "Application", combo: "[", when: "app" },
  { id: "appOutcome", label: "Set an outcome (then 1-6)", group: "Application", combo: "o", when: "app" },
  { id: "appRounds", label: "Edit every stage's rounds", group: "Application", combo: "r", when: "app" },
  { id: "appTasks", label: "Add or tick off tasks on the current stage", group: "Application", combo: "t", when: "app" },
  { id: "appRoundAdd", label: "Add a round to the current stage", group: "Application", combo: "+|=", when: "app" },
  { id: "appRoundRemove", label: "Remove the current stage's last round", group: "Application", combo: "-", when: "app" },
  { id: "appPosting", label: "Open the posting", group: "Application", combo: "p", when: "app" },
  { id: "appApprove", label: "Approve, when prepared", group: "Application", combo: "y", when: "app" },
  { id: "appReject", label: "Reject, when prepared", group: "Application", combo: "x", when: "app" },
  { id: "appArchive", label: "Archive or unarchive", group: "Application", combo: "a", when: "app" },
])
