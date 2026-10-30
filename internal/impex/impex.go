// Package impex parses the Pursuit CSV and writes Simplify's Job Tracker CSV.
package impex

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/pursuit/pursuit/internal/domain"
)

type Row struct {
	Line        int
	ExternalID  string
	Company     string
	Title       string
	Location    string
	Salary      string
	Fit         string
	Status      string
	AppliedDate string
	NextStep    string
	PostingURL  string
	Notes       string
	LastUpdated string
}

func ParseApplicationsCSV(r io.Reader) ([]Row, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	records, err := cr.ReadAll()
	if err != nil {
		return nil, domain.Validation("invalid csv: %s", err.Error()).WithHint("the file needs a header row and valid quoting")
	}
	if len(records) == 0 {
		return nil, domain.Validation("csv is empty").WithHint("include a header row: id,company,role,location,salary,fit,status,applied_date,next_step,posting_url,notes,last_updated")
	}
	index, err := headerIndex(records[0])
	if err != nil {
		return nil, err
	}
	if _, ok := index["company"]; !ok {
		return nil, domain.Validation("csv is missing a company column").WithHint("required columns are company and role")
	}
	if _, ok := index["role"]; !ok {
		return nil, domain.Validation("csv is missing a role column").WithHint("required columns are company and role")
	}
	out := make([]Row, 0, len(records)-1)
	for i, rec := range records[1:] {
		if isBlank(rec) {
			continue
		}
		row := Row{Line: i + 2}
		row.ExternalID = field(rec, index, "id")
		row.Company = field(rec, index, "company")
		row.Title = field(rec, index, "role")
		row.Location = field(rec, index, "location")
		row.Salary = field(rec, index, "salary")
		row.Fit = field(rec, index, "fit")
		row.Status = field(rec, index, "status")
		row.AppliedDate = field(rec, index, "applied_date")
		row.NextStep = field(rec, index, "next_step")
		row.PostingURL = field(rec, index, "posting_url")
		row.Notes = field(rec, index, "notes")
		row.LastUpdated = field(rec, index, "last_updated")
		out = append(out, row)
	}
	if len(out) > 5000 {
		return nil, domain.Validation("csv has %d rows", len(out)).WithHint("import at most 5000 rows at a time")
	}
	return out, nil
}

func headerIndex(header []string) (map[string]int, error) {
	idx := map[string]int{}
	for i, h := range header {
		key, ok := alias[normHeader(h)]
		if !ok {
			continue
		}
		if _, exists := idx[key]; exists {
			return nil, domain.Validation("duplicate csv column %q", h).WithHint("keep one column per field")
		}
		idx[key] = i
	}
	return idx, nil
}

func field(rec []string, idx map[string]int, key string) string {
	i, ok := idx[key]
	if !ok || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

func isBlank(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func normHeader(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(s, "\uFEFF"))
	return strings.ToLower(s)
}

var alias = map[string]string{
	"id": "id", "company": "company", "role": "role", "title": "role",
	"position": "role", "position title": "role", "location": "location",
	"salary": "salary", "fit": "fit", "status": "status",
	"applied_date": "applied_date", "applied date": "applied_date",
	"next_step": "next_step", "next step": "next_step",
	"posting_url": "posting_url", "posting url": "posting_url", "url": "posting_url",
	"posting link": "posting_url", "notes": "notes",
	"last_updated": "last_updated", "last updated": "last_updated",
}

type SimplifyRow struct {
	Company, Title, Location, JobType, Status, Notes, PostingURL string
}

func FromApplication(a domain.Application) SimplifyRow {
	job := a.JobType
	if strings.TrimSpace(job) == "" {
		job = "Full-time"
	}
	return SimplifyRow{
		Company:    a.Company,
		Title:      a.Title,
		Location:   a.Location,
		JobType:    job,
		Status:     domain.SimplifyStatus(a.EffectiveStatus()),
		Notes:      domain.SimplifyNotes(a.Fit, a.Salary, a.EffectiveStatus(), a.AppliedDateValue(), a.NextStep, a.Notes),
		PostingURL: a.PostingURL,
	}
}

func WriteSimplify(w io.Writer, rows []SimplifyRow) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"Company name", "Position Title", "Location", "Job type", "Status", "Notes", "Posting link"}); err != nil {
		return err
	}
	for _, row := range rows {
		if err := cw.Write([]string{row.Company, row.Title, row.Location, row.JobType, row.Status, row.Notes, row.PostingURL}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func RowError(line int, err error) error {
	if de, ok := domain.As(err); ok {
		cp := *de
		cp.Message = fmt.Sprintf("row %d: %s", line, de.Message)
		return &cp
	}
	return domain.Validation("row %d: %s", line, err.Error())
}
