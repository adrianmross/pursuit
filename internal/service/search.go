package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/embed"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

// searchSource is one thing smart search can find: a saved answer, a
// round's prep and notes, a text document. Link is the dashboard page.
type searchSource struct {
	Kind, ID, Title, Text, Link string
}

// Match is a source close in meaning to the query.
type Match struct {
	Source   string  `json:"source"`
	SourceID string  `json:"source_id"`
	Title    string  `json:"title"`
	Snippet  string  `json:"snippet"`
	Score    float64 `json:"score"`
	Link     string  `json:"link,omitempty"`
	// Answer is the saved answer itself, when Source is "answer".
	Answer string `json:"answer,omitempty"`
}

func humanizeKey(k string) string {
	k = strings.ReplaceAll(k, "_", " ")
	if k == "" {
		return k
	}
	return strings.ToUpper(k[:1]) + k[1:]
}

// searchSources gathers everything worth finding. New kinds (#96's parsed
// round questions, notes files, PDF text) are one more block here.
func (s *Service) searchSources(ctx context.Context, userID string) ([]searchSource, error) {
	var out []searchSource
	answers, err := s.DB.ListStanding(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, a := range answers {
		if strings.TrimSpace(a.Answer) == "" {
			continue
		}
		q := humanizeKey(a.Key)
		out = append(out, searchSource{Kind: "answer", ID: a.Key, Title: q, Text: q + "\n" + a.Answer, Link: "/profile/questions"})
	}
	rounds, err := s.DB.RoundTexts(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, r := range rounds {
		title := strings.TrimSpace(r.Company + " · " + strings.TrimSpace(r.Kind+" "+r.Stage))
		out = append(out, searchSource{Kind: "round", ID: r.ID, Title: title, Text: title + "\n" + r.Prep + "\n" + r.Notes, Link: "/app/" + r.ApplicationID})
	}
	docs, err := s.DB.ListDocuments(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, d := range docs {
		if !isText(d.ContentType) {
			continue
		}
		full, err := s.DB.GetDocument(ctx, userID, d.ID)
		if err == nil {
			full, err = s.withBody(ctx, full)
		}
		if err != nil {
			continue
		}
		out = append(out, searchSource{Kind: "document", ID: d.ID, Title: cmpOr(d.Label, d.Filename), Text: full.Body, Link: "/profile/documents"})
	}
	return out, nil
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

const chunkSize = 2000

// chunks splits text on paragraph breaks into pieces of about chunkSize.
func chunks(text string) []string {
	text = strings.TrimSpace(text)
	if len(text) <= chunkSize {
		return []string{text}
	}
	var out []string
	var cur strings.Builder
	for _, p := range strings.Split(text, "\n\n") {
		for len(p) > chunkSize {
			cut := strings.LastIndexAny(p[:chunkSize], " \n")
			if cut <= 0 {
				cut = chunkSize
			}
			if cur.Len() > 0 {
				out, cur = append(out, cur.String()), strings.Builder{}
			}
			out, p = append(out, p[:cut]), strings.TrimSpace(p[cut:])
		}
		if cur.Len()+len(p) > chunkSize && cur.Len() > 0 {
			out, cur = append(out, cur.String()), strings.Builder{}
		}
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(p)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func chunkHash(text string) string {
	h := sha256.Sum256([]byte(embed.Model + "|" + strconv.Itoa(embed.Dimensions) + "|" + text))
	return hex.EncodeToString(h[:])
}

// reindex brings the user's chunks in line with their sources, embedding
// only what changed and dropping what's gone.
// ponytail: runs on every search (cheap when nothing changed); hook writes into it if sources grow large.
func (s *Service) reindex(ctx context.Context, userID string) ([]sqlstore.Chunk, error) {
	srcs, err := s.searchSources(ctx, userID)
	if err != nil {
		return nil, err
	}
	have, err := s.DB.ListChunks(ctx, userID)
	if err != nil {
		return nil, err
	}
	key := func(source, id string, idx int) string { return source + "\x00" + id + "\x00" + strconv.Itoa(idx) }
	old := map[string]sqlstore.Chunk{}
	for _, c := range have {
		old[key(c.Source, c.SourceID, c.Idx)] = c
	}
	var keep []sqlstore.Chunk
	var todo []sqlstore.Chunk
	wanted := map[string]bool{}
	for _, src := range srcs {
		for i, text := range chunks(src.Text) {
			k := key(src.Kind, src.ID, i)
			wanted[k] = true
			c := sqlstore.Chunk{Source: src.Kind, SourceID: src.ID, Idx: i, Title: src.Title, Text: text, Hash: chunkHash(text)}
			if prev, ok := old[k]; ok && prev.Hash == c.Hash {
				prev.Title = src.Title
				keep = append(keep, prev)
				continue
			}
			todo = append(todo, c)
		}
	}
	if len(todo) > 0 {
		texts := make([]string, len(todo))
		for i, c := range todo {
			texts[i] = c.Text
		}
		vs, err := s.Embedder.Embed(ctx, texts)
		if err != nil {
			return nil, err
		}
		now := domain.FormatTime(s.now())
		for i := range todo {
			todo[i].Vector, todo[i].UpdatedAt = embed.Encode(vs[i]), now
			if err := s.DB.PutChunk(ctx, userID, todo[i]); err != nil {
				return nil, err
			}
		}
	}
	for k, c := range old {
		if !wanted[k] {
			if err := s.DB.DeleteChunk(ctx, userID, c.Source, c.SourceID, c.Idx); err != nil {
				return nil, err
			}
		}
	}
	return append(keep, todo...), nil
}

// MinScore is the weakest match worth showing.
const MinScore = 0.3

// rank scores every chunk against v and keeps each source's best.
// ponytail: brute-force cosine over the user's chunks (hundreds); a vector index if that reaches tens of thousands.
func rank(cs []sqlstore.Chunk, v []float32, only string, limit int) []Match {
	best := map[string]Match{}
	for _, c := range cs {
		if only != "" && c.Source != only {
			continue
		}
		cv, err := embed.Decode(c.Vector)
		if err != nil {
			continue
		}
		score := float64(embed.Dot(v, cv))
		k := c.Source + "\x00" + c.SourceID
		if m, ok := best[k]; ok && m.Score >= score {
			continue
		}
		best[k] = Match{Source: c.Source, SourceID: c.SourceID, Title: c.Title, Snippet: snippet(c.Text), Score: score}
	}
	out := make([]Match, 0, len(best))
	for _, m := range best {
		if m.Score >= MinScore {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func snippet(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 240 {
		cut := strings.LastIndex(text[:240], " ")
		if cut <= 0 {
			cut = 240
		}
		text = text[:cut] + "…"
	}
	return text
}

// Similar finds saved answers, rounds and documents close in meaning to q.
func (s *Service) Similar(ctx context.Context, q string, limit int) ([]Match, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	if err := s.RequireFeature(ctx, "smart_search"); err != nil {
		return nil, err
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return []Match{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	cs, err := s.reindex(ctx, a.UserID)
	if err != nil {
		return nil, err
	}
	vs, err := s.Embedder.Embed(ctx, []string{q})
	if err != nil {
		return nil, err
	}
	out := rank(cs, vs[0], "", limit)
	answers := map[string]string{}
	if rows, err := s.DB.ListStanding(ctx, a.UserID); err == nil {
		for _, r := range rows {
			answers[r.Key] = r.Answer
		}
	}
	links := map[string]string{"answer": "/profile/questions", "document": "/profile/documents"}
	rounds := map[string]string{}
	if rs, err := s.DB.RoundTexts(ctx, a.UserID); err == nil {
		for _, r := range rs {
			rounds[r.ID] = "/app/" + r.ApplicationID
		}
	}
	for i := range out {
		out[i].Link = links[out[i].Source]
		if out[i].Source == "round" {
			out[i].Link = rounds[out[i].SourceID]
		}
		if out[i].Source == "answer" {
			out[i].Answer = answers[out[i].SourceID]
		}
	}
	return out, nil
}

// SuggestThreshold is how close a saved answer must be to be suggested for a form question.
const SuggestThreshold = 0.55

// suggestAnswers records, for each unknown question, the closest saved
// answer. Best effort: smart search off or failing leaves them as they were.
func (s *Service) suggestAnswers(ctx context.Context, qs []QuestionView) {
	a, err := s.actor(ctx, false)
	if err != nil || s.RequireFeature(ctx, "smart_search") != nil {
		return
	}
	var idx []int
	var prompts []string
	for i, q := range qs {
		if q.Unknown {
			idx, prompts = append(idx, i), append(prompts, q.Prompt)
		}
	}
	cs, err := s.reindex(ctx, a.UserID)
	if err != nil || len(cs) == 0 {
		return
	}
	vs, err := s.Embedder.Embed(ctx, prompts)
	if err != nil {
		return
	}
	answers := map[string]string{}
	if rows, err := s.DB.ListStanding(ctx, a.UserID); err == nil {
		for _, r := range rows {
			answers[r.Key] = r.Answer
		}
	}
	for n, i := range idx {
		m := rank(cs, vs[n], "answer", 1)
		if len(m) == 0 || m[0].Score < SuggestThreshold || answers[m[0].SourceID] == "" {
			continue
		}
		q := &qs[i]
		q.SuggestedAnswer, q.SuggestedSource, q.SuggestedScore = answers[m[0].SourceID], m[0].SourceID, m[0].Score
		_ = s.DB.SuggestAnswer(ctx, a.UserID, q.ID, q.SuggestedAnswer, q.SuggestedSource, q.SuggestedScore)
	}
}
