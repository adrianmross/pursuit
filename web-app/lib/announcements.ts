import { latestAnnouncement, type Announcement } from "@adrianmross/appkit-ui"
import authored from "@/content/announcements.json"
import generated from "@/lib/releases.generated.json"

// The newest of the written announcements and the ones generated from
// CHANGELOG.md, for useAnnouncement().
export const latest = latestAnnouncement(authored as Announcement[], generated as Announcement[])
