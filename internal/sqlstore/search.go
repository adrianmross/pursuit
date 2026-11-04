package sqlstore

import "context"

// Chunk is one embedded piece of a searchable source.
type Chunk struct {
	Source    string
	SourceID  string
	Idx       int
	Title     string
	Text      string
	Hash      string
	Vector    string
	UpdatedAt string
}

func (s *Store) ListChunks(ctx context.Context, userID string) ([]Chunk, error) {
	rows, err := s.query(ctx, `SELECT source, source_id, idx, title, text, hash, vector, updated_at FROM search_chunks WHERE user_id=?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Chunk{}
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.Source, &c.SourceID, &c.Idx, &c.Title, &c.Text, &c.Hash, &c.Vector, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) PutChunk(ctx context.Context, userID string, c Chunk) error {
	_, err := s.exec(ctx, `INSERT INTO search_chunks (user_id, source, source_id, idx, title, text, hash, vector, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, source, source_id, idx) DO UPDATE SET title=excluded.title, text=excluded.text, hash=excluded.hash, vector=excluded.vector, updated_at=excluded.updated_at`,
		userID, c.Source, c.SourceID, c.Idx, c.Title, c.Text, c.Hash, c.Vector, c.UpdatedAt)
	return err
}

func (s *Store) DeleteChunk(ctx context.Context, userID, source, sourceID string, idx int) error {
	_, err := s.exec(ctx, `DELETE FROM search_chunks WHERE user_id=? AND source=? AND source_id=? AND idx=?`, userID, source, sourceID, idx)
	return err
}

// RoundText is one interview round's own words, with what it's for.
type RoundText struct {
	ID, ApplicationID, Company, Title, Stage, Kind, Prep, Notes string
}

// RoundTexts is every round with prep or notes, newest first.
func (s *Store) RoundTexts(ctx context.Context, userID string) ([]RoundText, error) {
	rows, err := s.query(ctx, `SELECT i.id, i.application_id, c.name, r.title, i.stage, i.kind, i.prep, i.notes
		FROM interviews i JOIN applications a ON a.id = i.application_id JOIN roles r ON r.id = a.role_id JOIN companies c ON c.id = r.company_id
		WHERE i.user_id=? AND (i.prep <> '' OR i.notes <> '') ORDER BY i.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RoundText{}
	for rows.Next() {
		var t RoundText
		if err := rows.Scan(&t.ID, &t.ApplicationID, &t.Company, &t.Title, &t.Stage, &t.Kind, &t.Prep, &t.Notes); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SuggestAnswer records the closest saved answer for a question.
func (s *Store) SuggestAnswer(ctx context.Context, userID, questionID, answer, source string, score float64) error {
	_, err := s.exec(ctx, `UPDATE questions SET suggested_answer=?, suggested_source=?, suggested_score=? WHERE user_id=? AND id=?`, answer, source, score, userID, questionID)
	return err
}
