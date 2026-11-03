package domain

import (
	"reflect"
	"testing"
)

const transcript = `## Prep
Read the Polaris post.

## Questions
### Why Snowflake?
Query engine work at scale.

### Tell me about a migration you led? ##
Postgres to Spanner.

` + "```" + `
# not a heading?
` + "```" + `

## Notes
Went well.
`

func TestParseRoundQuestions(t *testing.T) {
	got := ParseRoundQuestions(transcript)
	want := []RoundQuestion{
		{Prompt: "Why Snowflake?", Answer: "Query engine work at scale."},
		{Prompt: "Tell me about a migration you led?", Answer: "Postgres to Spanner.\n\n```\n# not a heading?\n```"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestSetRoundQuestion(t *testing.T) {
	// Replace keeps everything else.
	md := SetRoundQuestion(transcript, "why snowflake", "Scale.")
	if q := ParseRoundQuestions(md); q[0].Answer != "Scale." || len(q) != 2 {
		t.Fatalf("replace: %#v\n%s", q, md)
	}
	// Insert lands at the end of Questions, before Notes.
	md = SetRoundQuestion(transcript, "Any questions for us", "Team size.")
	q := ParseRoundQuestions(md)
	if len(q) != 3 || q[2].Prompt != "Any questions for us?" || q[2].Answer != "Team size." {
		t.Fatalf("insert: %#v\n%s", q, md)
	}
	if ParseRoundQuestions(SetSection(md, "Notes", "x"))[2].Answer != "Team size." {
		t.Fatalf("insert landed after Notes:\n%s", md)
	}
	// No Questions section yet.
	md = SetRoundQuestion("Some notes.", "Why us?", "Because.")
	if md != "Some notes.\n\n## Questions\n\n### Why us?\nBecause.\n" {
		t.Fatalf("create: %q", md)
	}
}

func TestSetSection(t *testing.T) {
	md := SetSection(transcript, "Prep", "Read the 10-K.")
	if ParseRoundQuestions(md)[0].Answer != "Query engine work at scale." {
		t.Fatalf("prep replace touched questions:\n%s", md)
	}
	if got := SetSection("", "Prep", "Read."); got != "## Prep\nRead.\n" {
		t.Fatalf("create: %q", got)
	}
	if got := AppendNotes("a", "b"); got != "a\n\nb\n" {
		t.Fatalf("append: %q", got)
	}
	// Appending after a question opens a Notes section instead of extending
	// the answer; a second append stays in it.
	md = AppendNotes(AppendNotes("### Why us?\nScale.", "Went well."), "Next: final.")
	if q := ParseRoundQuestions(md); len(q) != 1 || q[0].Answer != "Scale." {
		t.Fatalf("append after question: %#v\n%s", q, md)
	}
	if md != "### Why us?\nScale.\n\n## Notes\nWent well.\n\nNext: final.\n" {
		t.Fatalf("append after question: %q", md)
	}
}
