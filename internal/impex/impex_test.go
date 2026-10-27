package impex

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/pursuit/pursuit/internal/domain"
)

func TestFixtureSimplifyMapping(t *testing.T) {
	in, err := os.Open("../../testdata/applications.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	rows, err := ParseApplicationsCSV(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	got := make([][]string, 0, len(rows)+1)
	got = append(got, []string{"Company name", "Position Title", "Location", "Job type", "Status", "Notes", "Posting link"})
	for _, row := range rows {
		status, err := domain.ParseStatus(row.Status)
		if err != nil {
			t.Fatal(err)
		}
		fit, err := domain.ParseFit(row.Fit)
		if err != nil {
			t.Fatal(err)
		}
		app := domain.Application{
			Company: row.Company, Title: row.Title, Location: row.Location,
			Salary: row.Salary, Fit: fit, Status: status, NextStep: row.NextStep,
			Notes: row.Notes, PostingURL: row.PostingURL, JobType: "Full-time",
		}
		if row.AppliedDate != "" {
			d := row.AppliedDate
			app.AppliedDate = &d
		}
		s := FromApplication(app)
		got = append(got, []string{s.Company, s.Title, s.Location, s.JobType, s.Status, s.Notes, s.PostingURL})
	}
	f, err := os.Open("../../testdata/simplify_import.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	want, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("records %d vs %d", len(got), len(want))
	}
	for i := range want {
		if strings.Join(got[i], "\x1f") != strings.Join(want[i], "\x1f") {
			t.Fatalf("row %d\n got %#v\nwant %#v", i, got[i], want[i])
		}
	}
}
