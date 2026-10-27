package impex

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/pursuit/pursuit/internal/domain"
)

// Posting is a role row from CSV, JSON, or a markdown table.
type Posting struct {
	ExternalID  string
	Company     string
	Title       string
	Location    string
	Salary      string
	Fit         string
	Status      string
	AppliedDate string
	NextStep    string
	URL         string
	Notes       string
	LastUpdated string
	Line        int
}

func PostingsFromCSV(r io.Reader) ([]Posting, error) {
	rows, err := ParseApplicationsCSV(r)
	if err != nil {
		return nil, err
	}
	out := make([]Posting, len(rows))
	for i, row := range rows {
		out[i] = Posting{
			Line: row.Line, ExternalID: row.ExternalID, Company: row.Company, Title: row.Title,
			Location: row.Location, Salary: row.Salary, Fit: row.Fit, Status: row.Status,
			AppliedDate: row.AppliedDate, NextStep: row.NextStep, URL: row.PostingURL,
			Notes: row.Notes, LastUpdated: row.LastUpdated,
		}
	}
	return out, nil
}

func ParsePostingsJSON(r io.Reader) ([]Posting, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var raw []struct {
		Company     string `json:"company"`
		Title       string `json:"title"`
		Role        string `json:"role"`
		Location    string `json:"location"`
		Salary      string `json:"salary"`
		Fit         string `json:"fit"`
		Status      string `json:"status"`
		URL         string `json:"url"`
		PostingURL  string `json:"posting_url"`
		Notes       string `json:"notes"`
		NextStep    string `json:"next_step"`
		AppliedDate string `json:"applied_date"`
		ExternalID  string `json:"id"`
	}
	if err := dec.Decode(&raw); err != nil {
		return nil, domain.Validation("invalid json: %s", err.Error()).WithHint(`send a JSON array of {"company","title","url"} objects`)
	}
	out := make([]Posting, 0, len(raw))
	for i, row := range raw {
		title := row.Title
		if title == "" {
			title = row.Role
		}
		u := row.URL
		if u == "" {
			u = row.PostingURL
		}
		out = append(out, Posting{
			Line: i + 1, ExternalID: row.ExternalID, Company: cleanCompany(row.Company), Title: strings.TrimSpace(title),
			Location: row.Location, Salary: row.Salary, Fit: row.Fit, Status: row.Status,
			URL: strings.TrimSpace(u), Notes: row.Notes, NextStep: row.NextStep, AppliedDate: row.AppliedDate,
		})
	}
	return out, nil
}

var (
	linkRe   = regexp.MustCompile(`https?://[^\s)|]+`)
	moneyRe  = regexp.MustCompile(`\$[\d.,]+[KkMm]?(?:\s*[–-]\s*\$[\d.,]+[KkMm]?)?`)
	fitRe    = regexp.MustCompile(`(?i)\*{0,2}(strong|reach|maybe)\*{0,2}`)
	bulletRe = regexp.MustCompile(`^[-*]\s+(.+)$`)
)

// ParseMarkdownTables reads GitHub-style tables and "also live" bullets.
// Press links are ignored. A heading such as "## 1. Anduril" supplies the
// company when a table has no company column.
func ParseMarkdownTables(r io.Reader) ([]Posting, error) {
	b, err := io.ReadAll(io.LimitReader(r, 8<<20))
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	var out []Posting
	seen := map[string]bool{}
	companyHint := ""
	var table []string
	flush := func() {
		if len(table) == 0 {
			return
		}
		out = append(out, parseTable(table, companyHint, seen)...)
		table = nil
	}
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "##") {
			flush()
			companyHint = headingCompany(trim)
			continue
		}
		if strings.HasPrefix(trim, "|") {
			table = append(table, trim)
			continue
		}
		flush()
		if m := bulletRe.FindStringSubmatch(trim); m != nil {
			if p, ok := parseBullet(m[1], i+1); ok && !seen[p.URL] && isJobURL(p.URL) {
				seen[p.URL] = true
				out = append(out, p)
			}
		}
	}
	flush()
	return out, nil
}

func parseTable(lines []string, companyHint string, seen map[string]bool) []Posting {
	if len(lines) < 2 {
		return nil
	}
	header := splitRow(lines[0])
	start := 1
	if len(lines) > 1 && isSep(lines[1]) {
		start = 2
	}
	col := map[string]int{}
	for i, h := range header {
		col[normHeader(h)] = i
	}
	urlCol := pick(col, "url", "posting url", "posting link", "link")
	if urlCol < 0 {
		return nil
	}
	var out []Posting
	for n, line := range lines[start:] {
		cells := splitRow(line)
		rawURL := cell(cells, urlCol)
		urls := jobURLs(rawURL)
		if len(urls) == 0 {
			urls = jobURLs(line)
		}
		if len(urls) == 0 {
			continue
		}
		company := cell(cells, pick(col, "company"))
		if company == "" {
			company = companyHint
		}
		company = cleanCompany(company)
		title := cell(cells, pick(col, "role", "title", "position", "position title"))
		location := cell(cells, pick(col, "location", "location / on-site", "location / on site"))
		salary := cell(cells, pick(col, "base salary", "salary", "base", "posted range"))
		fitCell := cell(cells, pick(col, "fit", "fit / flag"))
		fit, fitNotes := splitFit(fitCell)
		if salary == "" {
			salary = firstMoney(fitCell + " " + line)
		}
		if salary == "Not listed" || strings.EqualFold(salary, "not shown on the page") {
			salary = "Not listed"
		}
		notes := strings.TrimSpace(fitNotes)
		for i, u := range urls {
			if seen[u] || !isJobURL(u) {
				continue
			}
			seen[u] = true
			item := Posting{
				Line: n + 1, Company: company, Title: strings.Trim(title, "* "),
				Location: plain(location), Salary: plainSalary(salary), Fit: fit, URL: u, Notes: notes,
			}
			if i > 0 && item.Title != "" {
				item.Notes = strings.TrimSpace(item.Notes + " additional posting")
			}
			if item.Company == "" || item.Title == "" {
				if c, t, ok := guessFromLine(line); ok {
					if item.Company == "" {
						item.Company = c
					}
					if item.Title == "" {
						item.Title = t
					}
				}
			}
			if item.Company == "" || item.Title == "" {
				continue
			}
			out = append(out, item)
		}
	}
	return out
}

func parseBullet(body string, line int) (Posting, bool) {
	urls := jobURLs(body)
	if len(urls) != 1 {
		return Posting{}, false
	}
	u := urls[0]
	text := strings.TrimSpace(strings.Replace(body, u, "", 1))
	text = strings.TrimRight(text, " .)")
	fit, text := stripFit(text)
	comma := strings.Index(text, ",")
	if comma < 1 {
		return Posting{}, false
	}
	company := cleanCompany(text[:comma])
	rest := strings.TrimSpace(text[comma+1:])
	title := rest
	location := ""
	notes := ""
	if i := strings.Index(rest, ":"); i >= 0 {
		title = strings.TrimSpace(rest[:i])
		location = strings.TrimSpace(rest[i+1:])
		if j := strings.Index(location, "."); j >= 0 {
			notes = strings.TrimSpace(location[j+1:])
			location = strings.TrimSpace(location[:j])
		}
	}
	if company == "" || title == "" {
		return Posting{}, false
	}
	return Posting{
		Line: line, Company: company, Title: plain(title), Location: plain(location),
		Salary: firstMoney(text), Fit: fit, URL: u, Notes: notes,
	}, true
}

func stripFit(s string) (string, string) {
	s = plain(s)
	m := fitRe.FindStringSubmatchIndex(s)
	if m == nil {
		return "", strings.TrimSpace(s)
	}
	word := strings.ToLower(s[m[2]:m[3]])
	cleaned := strings.TrimSpace(s[:m[0]] + " " + s[m[1]:])
	cleaned = strings.Trim(cleaned, " :;.-()")
	return word, strings.Join(strings.Fields(cleaned), " ")
}

func jobURLs(s string) []string {
	found := linkRe.FindAllString(s, -1)
	var out []string
	seen := map[string]bool{}
	for _, u := range found {
		u = strings.TrimRight(u, ".,);")
		if !isJobURL(u) || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

func isJobURL(raw string) bool {
	u, err := domain.ParseURL(raw)
	if err != nil || u == "" {
		return false
	}
	host := strings.ToLower(hostOf(u))
	switch host {
	case "businesswire.com", "www.businesswire.com", "cnbc.com", "www.cnbc.com",
		"techcrunch.com", "www.techcrunch.com", "dealroom.co", "www.dealroom.co",
		"bloomberg.com", "www.bloomberg.com":
		return false
	default:
		return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
	}
}

func hostOf(raw string) string {
	u, err := regexp.Compile(`^https?://([^/]+)`)
	if err != nil {
		return ""
	}
	m := u.FindStringSubmatch(raw)
	if m == nil {
		return ""
	}
	return m[1]
}

func headingCompany(line string) string {
	line = strings.TrimLeft(line, "#")
	line = strings.TrimSpace(line)
	line = regexp.MustCompile(`^\d+\.\s*`).ReplaceAllString(line, "")
	if i := strings.Index(line, "("); i > 0 {
		line = line[:i]
	}
	if i := strings.Index(line, "—"); i > 0 {
		line = line[:i]
	}
	if i := strings.Index(line, " - "); i > 0 {
		line = line[:i]
	}
	// "Sierra & Replit" is not a single company; tables in that section have a company column.
	if strings.Contains(line, "&") || strings.Contains(strings.ToLower(line), "coverage") || strings.Contains(strings.ToLower(line), "startup") {
		return ""
	}
	return cleanCompany(line)
}

func cleanCompany(s string) string {
	s = plain(s)
	s = regexp.MustCompile(`(?i)\s*\(startup\)\s*`).ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func plain(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

func plainSalary(s string) string {
	s = plain(s)
	if m := moneyRe.FindString(s); m != "" && !strings.Contains(s, "–") && !strings.Contains(s, "-") {
		return m
	}
	if i := strings.Index(s, "("); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func firstMoney(s string) string {
	s = plain(s)
	m := moneyRe.FindString(s)
	return strings.TrimSpace(m)
}

func splitFit(s string) (fit, notes string) {
	s = plain(s)
	m := fitRe.FindStringSubmatchIndex(s)
	if m == nil {
		return "", strings.TrimSpace(s)
	}
	word := strings.ToLower(s[m[2]:m[3]])
	rest := strings.TrimSpace(s[m[1]:])
	rest = strings.TrimLeft(rest, ":;- ")
	return word, rest
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.Trim(line, "|")
	r := csv.NewReader(strings.NewReader(line))
	r.Comma = '|'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	rec, err := r.Read()
	if err != nil {
		return strings.Split(line, "|")
	}
	for i := range rec {
		rec[i] = strings.TrimSpace(rec[i])
	}
	return rec
}

func isSep(line string) bool {
	for _, r := range line {
		if r != '|' && r != '-' && r != ':' && r != ' ' {
			return false
		}
	}
	return strings.Contains(line, "-")
}

func pick(col map[string]int, names ...string) int {
	for _, n := range names {
		if i, ok := col[n]; ok {
			return i
		}
	}
	return -1
}

func cell(cells []string, i int) string {
	if i < 0 || i >= len(cells) {
		return ""
	}
	return strings.TrimSpace(cells[i])
}

func guessFromLine(line string) (company, title string, ok bool) {
	plainLine := plain(line)
	if !strings.Contains(plainLine, "|") {
		return "", "", false
	}
	return "", "", false
}

// NormalizeKey folds a question prompt into a standing-answer key.
func NormalizeKey(s string) string {
	var b strings.Builder
	prev := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prev = false
			continue
		}
		if !prev && b.Len() > 0 {
			b.WriteByte('_')
			prev = true
		}
	}
	return strings.Trim(b.String(), "_")
}

// DecodeCSVBytes is a small helper for tests that already hold bytes.
func DecodeCSVBytes(b []byte) ([]Posting, error) {
	return PostingsFromCSV(bytes.NewReader(b))
}
