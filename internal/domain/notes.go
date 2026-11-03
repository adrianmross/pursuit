package domain

import (
	"encoding/json"
	"strings"
	"time"
)

// A round's notes are one markdown document. A question is a heading that
// ends in "?"; its answer is the text under it, up to the next heading.
type RoundQuestion struct {
	Prompt string `json:"prompt"`
	Answer string `json:"answer"`
}

// BankQuestion is a round question with where it was asked, for the shared
// question bank across applications.
type BankQuestion struct {
	RoundQuestion
	RoundID       string    `json:"round_id"`
	ApplicationID string    `json:"application_id"`
	Company       string    `json:"company"`
	Title         string    `json:"title"`
	Stage         string    `json:"stage"`
	Kind          string    `json:"kind"`
	UpdatedAt     time.Time `json:"updated_at"`
}

var BankQuestionFields = []string{"prompt", "answer", "round_id", "application_id", "company", "title", "stage", "kind", "updated_at"}

// StagePlan is one application's own process: its names for the round
// stages (Prepared through Final), keyed by the stage they stand for, and the
// stages it skips. The statuses underneath never change.
type StagePlan struct {
	Labels map[string]string `json:"labels,omitempty"`
	Off    []string          `json:"off,omitempty"`
}

// Normalize checks keys are round stages and labels are short, dropping empty
// labels and duplicate offs. A plan with nothing left is nil.
func (p *StagePlan) Normalize() (*StagePlan, error) {
	if p == nil {
		return nil, nil
	}
	out := &StagePlan{Labels: map[string]string{}}
	for k, v := range p.Labels {
		key, err := ParseInterviewStage(k)
		if err != nil {
			return nil, err
		}
		v = strings.Join(strings.Fields(v), " ")
		if len([]rune(v)) > 40 {
			return nil, Validation("stage label %q is longer than 40 characters", v)
		}
		if v != "" {
			out.Labels[key] = v
		}
	}
	seen := map[string]bool{}
	for _, k := range p.Off {
		key, err := ParseInterviewStage(k)
		if err != nil {
			return nil, err
		}
		if !seen[key] {
			seen[key] = true
			out.Off = append(out.Off, key)
		}
	}
	if len(out.Labels) == 0 && len(out.Off) == 0 {
		return nil, nil
	}
	if len(out.Labels) == 0 {
		out.Labels = nil
	}
	return out, nil
}

// Encode is the stored form; nil stores as empty.
func (p *StagePlan) Encode() string {
	if p == nil {
		return ""
	}
	b, _ := json.Marshal(p)
	return string(b)
}

type heading struct {
	line  int // index into the document's lines
	level int
	text  string
}

// headings lists ATX headings outside fenced code blocks.
func headings(lines []string) []heading {
	var out []heading
	fence := ""
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if fence != "" {
			if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = t[:3]
			continue
		}
		level := len(t) - len(strings.TrimLeft(t, "#"))
		if level < 1 || level > 6 || (len(t) > level && t[level] != ' ') {
			continue
		}
		text := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(t[level:]), "#"))
		out = append(out, heading{line: i, level: level, text: text})
	}
	return out
}

// sameHeading compares heading text the way people retype it: case, spacing
// and a trailing "?" don't matter.
func sameHeading(a, b string) bool {
	f := func(s string) string {
		return strings.Join(strings.Fields(strings.ToLower(strings.TrimRight(s, "? "))), " ")
	}
	return f(a) == f(b)
}

// sectionEnd is the line where the body under hs[i] stops: the next heading.
func sectionEnd(hs []heading, i, n int) int {
	if i+1 < len(hs) {
		return hs[i+1].line
	}
	return n
}

func ParseRoundQuestions(md string) []RoundQuestion {
	lines := strings.Split(md, "\n")
	hs := headings(lines)
	out := []RoundQuestion{}
	for i, h := range hs {
		if !strings.HasSuffix(h.text, "?") {
			continue
		}
		body := strings.Join(lines[h.line+1:sectionEnd(hs, i, len(lines))], "\n")
		out = append(out, RoundQuestion{Prompt: h.text, Answer: strings.TrimSpace(body)})
	}
	return out
}

// SetSection replaces the body under the first heading matching title, or
// appends a level-2 section when there is none.
func SetSection(md, title, body string) string {
	lines := strings.Split(md, "\n")
	hs := headings(lines)
	for i, h := range hs {
		if sameHeading(h.text, title) {
			return replaceBody(lines, h.line, sectionEnd(hs, i, len(lines)), body)
		}
	}
	return appendBlock(md, "## "+title+"\n"+strings.TrimSpace(body))
}

// SetRoundQuestion replaces a question's answer, or adds the question at the
// end of the "Questions" section, creating that section when missing.
func SetRoundQuestion(md, prompt, answer string) string {
	prompt = strings.TrimSpace(prompt)
	if !strings.HasSuffix(prompt, "?") {
		prompt += "?"
	}
	lines := strings.Split(md, "\n")
	hs := headings(lines)
	for i, h := range hs {
		if sameHeading(h.text, prompt) {
			return replaceBody(lines, h.line, sectionEnd(hs, i, len(lines)), answer)
		}
	}
	block := "### " + prompt + "\n" + strings.TrimSpace(answer)
	for i, h := range hs {
		if h.level <= 2 && sameHeading(h.text, "Questions") {
			// The section runs until the next heading at its level or above.
			end := len(lines)
			for _, next := range hs[i+1:] {
				if next.level <= h.level {
					end = next.line
					break
				}
			}
			before := strings.TrimRight(strings.Join(lines[:end], "\n"), "\n")
			after := strings.Join(lines[end:], "\n")
			if after != "" {
				return before + "\n\n" + block + "\n\n" + after
			}
			return before + "\n\n" + block + "\n"
		}
	}
	return appendBlock(md, "## Questions\n\n"+block)
}

// DemoteHeadings moves every heading outside a code fence one level down, so
// a round's document can sit under its own heading in the compiled notes.
func DemoteHeadings(md string) string {
	lines := strings.Split(md, "\n")
	fence := false
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = !fence
			continue
		}
		if !fence && strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "######") {
			lines[i] = "#" + t
		}
	}
	return strings.Join(lines, "\n")
}

// AppendNotes adds text at the end as its own paragraph. When the document
// ends inside a question, the text goes under a new "Notes" heading so it
// never becomes part of that question's answer.
func AppendNotes(md, text string) string {
	text = strings.TrimSpace(text)
	if hs := headings(strings.Split(md, "\n")); len(hs) > 0 && strings.HasSuffix(hs[len(hs)-1].text, "?") {
		text = "## Notes\n" + text
	}
	return appendBlock(md, text)
}

func replaceBody(lines []string, at, end int, body string) string {
	out := append([]string{}, lines[:at+1]...)
	if b := strings.TrimSpace(body); b != "" {
		out = append(out, b)
	}
	if end < len(lines) {
		out = append(out, "")
		out = append(out, lines[end:]...)
	}
	return strings.Join(out, "\n")
}

func appendBlock(md, block string) string {
	md = strings.TrimRight(md, "\n ")
	if md == "" {
		return block + "\n"
	}
	return md + "\n\n" + block + "\n"
}
