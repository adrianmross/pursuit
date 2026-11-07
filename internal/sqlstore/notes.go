package sqlstore

import (
	"context"
	"database/sql"

	"github.com/pursuit/pursuit/internal/domain"
)

const noteCols = `id, user_id, application_id, name, body, version, created_at, updated_at`

func scanNote(row scanner) (domain.Note, error) {
	var n domain.Note
	var created, updated string
	if err := row.Scan(&n.ID, &n.UserID, &n.ApplicationID, &n.Name, &n.Body, &n.Version, &created, &updated); err != nil {
		return domain.Note{}, err
	}
	var err error
	if n.CreatedAt, err = parseTime(created); err != nil {
		return domain.Note{}, err
	}
	n.UpdatedAt, err = parseTime(updated)
	n.Questions = domain.ParseRoundQuestions(n.Body)
	return n, err
}

func (s *Store) CreateNote(ctx context.Context, n domain.Note) error {
	_, err := s.exec(ctx, `INSERT INTO notes (`+noteCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.UserID, n.ApplicationID, n.Name, n.Body, n.Version, formatTime(n.CreatedAt), formatTime(n.UpdatedAt))
	return mapWriteErr(err)
}

func (s *Store) GetNote(ctx context.Context, userID, id string) (domain.Note, error) {
	n, err := scanNote(s.queryRow(ctx, `SELECT `+noteCols+` FROM notes WHERE user_id=? AND id=?`, userID, id))
	if err == sql.ErrNoRows {
		return domain.Note{}, domain.NotFound("note")
	}
	return n, err
}

// ListNotes returns an application's notes files in the order they were made.
func (s *Store) ListNotes(ctx context.Context, userID, appID string) ([]domain.Note, error) {
	rows, err := s.query(ctx, `SELECT `+noteCols+` FROM notes WHERE user_id=? AND application_id=? ORDER BY created_at, id`, userID, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Note{}
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) noteVersion(ctx context.Context, userID, id string) func() (int, error) {
	return func() (int, error) {
		cur, err := s.GetNote(ctx, userID, id)
		if err != nil {
			return 0, err
		}
		return cur.Version, nil
	}
}

func (s *Store) UpdateNote(ctx context.Context, n domain.Note) error {
	res, err := s.exec(ctx, `UPDATE notes SET name=?, body=?, version=version+1, updated_at=? WHERE id=? AND user_id=? AND version=?`,
		n.Name, n.Body, formatTime(n.UpdatedAt), n.ID, n.UserID, n.Version)
	if err != nil {
		return mapWriteErr(err)
	}
	return changed(res, s.noteVersion(ctx, n.UserID, n.ID), "note")
}

func (s *Store) DeleteNote(ctx context.Context, userID, id string, version int) error {
	res, err := s.exec(ctx, `DELETE FROM notes WHERE id=? AND user_id=? AND version=?`, id, userID, version)
	if err != nil {
		return err
	}
	return changed(res, s.noteVersion(ctx, userID, id), "note")
}
