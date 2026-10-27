package domain

import "testing"

func TestParseStatusAndFit(t *testing.T) {
	cases := map[string]string{
		"Applied":     StatusApplied,
		"in progress": StatusInProgress,
		"In-Progress": StatusInProgress,
		"to-apply":    StatusToApply,
		"Prepared":    StatusAwaitingReview,
		"needs input": StatusNeedsInput,
		"Ghosted":     StatusGhosted,
	}
	for in, want := range cases {
		got, err := ParseStatus(in)
		if err != nil || got != want {
			t.Fatalf("ParseStatus(%q)=%q,%v want %q", in, got, err, want)
		}
	}
	if _, err := ParseStatus("nope"); err == nil {
		t.Fatal("expected invalid status")
	}
	fit, err := ParseFit("Strong")
	if err != nil || fit != "strong" || FitLabel(fit) != "Strong" {
		t.Fatalf("fit=%q err=%v", fit, err)
	}
}

func TestParseSalary(t *testing.T) {
	min, max := ParseSalary("$100K–$150K + equity")
	if min == nil || max == nil || *min != 10000000 || *max != 15000000 {
		t.Fatalf("range min=%v max=%v", min, max)
	}
	min, max = ParseSalary("$120.5K–$180.5K")
	if min == nil || max == nil || *min != 12050000 || *max != 18050000 {
		t.Fatalf("decimal min=%v max=%v", min, max)
	}
	min, max = ParseSalary("Not listed")
	if min != nil || max != nil {
		t.Fatalf("not listed min=%v max=%v", min, max)
	}
}

func TestInferSource(t *testing.T) {
	cases := map[string]string{
		"https://jobs.ashbyhq.com/openai/abc":                                 SourceAshby,
		"https://job-boards.greenhouse.io/tailscale/jobs/1":                   SourceGreenhouse,
		"https://databricks.com/company/careers/open-positions/job?gh_jid=8": SourceGreenhouse,
		"https://example.com/careers/platform-engineer":              SourceCompanySite,
		"https://jobs.lever.co/acme/1":                                        SourceLever,
	}
	for raw, want := range cases {
		if got := InferSource(raw); got != want {
			t.Fatalf("InferSource(%s)=%s want %s", raw, got, want)
		}
	}
}

func TestSimplifyNotes(t *testing.T) {
	got := SimplifyNotes("strong", "$100K–$140K", StatusApplied, "2026-01-15", "Send a sample follow-up", "Example note only")
	want := "Strong fit; salary $100K–$140K; applied 2026-01-15; next: Send a sample follow-up; Example note only"
	if got != want {
		t.Fatalf("notes\n got %s\nwant %s", got, want)
	}
	got = SimplifyNotes("strong", "$120.5K–$180.5K", StatusInProgress, "", "Wait for a sample reply", "Example in-progress note")
	want = "Strong fit; salary $120.5K–$180.5K; status: In progress; next: Wait for a sample reply; Example in-progress note"
	if got != want {
		t.Fatalf("in progress notes\n got %s\nwant %s", got, want)
	}
	if SimplifyStatus(StatusInProgress) != "Saved" || SimplifyStatus(StatusApplied) != "Applied" {
		t.Fatal("simplify status mapping")
	}
}

func TestCursorRoundTrip(t *testing.T) {
	s := EncodeCursor("-updated_at", "2026-10-26T00:00:00.000000000Z", "abc")
	c, err := DecodeCursor(s)
	if err != nil || c.Sort != "-updated_at" || c.ID != "abc" {
		t.Fatalf("cursor %+v err=%v", c, err)
	}
	if err := CheckCursorSort(c, "name"); err == nil {
		t.Fatal("expected sort mismatch")
	}
}
