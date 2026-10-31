// Package skills embeds the Agent Skills in this directory. skills/pursuit/SKILL.md
// is the one copy: `npx skills add adrianmross/pursuit` installs it and the
// server serves it at /skill.md. This file sits outside skills/pursuit so the
// skills CLI does not install it.
package skills

import _ "embed"

//go:embed pursuit/SKILL.md
var Pursuit []byte
