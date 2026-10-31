package sqlstore

import (
	"context"
	"database/sql"

	"github.com/pursuit/pursuit/internal/domain"
)

// Document is a user-level file: the base résumé prepare attaches, a
// cover-letter template, a LinkedIn PDF export. Body is base64 for binary
// content, as with Artifact.
type Document struct {
	ID          string
	UserID      string
	Kind        string
	Filename    string
	ContentType string
	Body        string
	CreatedAt   string
}

func (s *Store) CreateDocument(ctx context.Context, d Document) error {
	_, err := s.exec(ctx, `INSERT INTO documents (id, user_id, kind, filename, content_type, body, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.UserID, d.Kind, d.Filename, d.ContentType, d.Body, d.CreatedAt)
	return err
}

func (s *Store) ListDocuments(ctx context.Context, userID string) ([]Document, error) {
	rows, err := s.query(ctx, `SELECT id, user_id, kind, filename, content_type, created_at FROM documents WHERE user_id=? ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.UserID, &d.Kind, &d.Filename, &d.ContentType, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDocument(ctx context.Context, userID, id string) (Document, error) {
	return s.oneDocument(ctx, `WHERE user_id=? AND id=?`, userID, id)
}

// LatestDocument returns the newest document of kind, or not_found.
func (s *Store) LatestDocument(ctx context.Context, userID, kind string) (Document, error) {
	return s.oneDocument(ctx, `WHERE user_id=? AND kind=? ORDER BY created_at DESC, id DESC LIMIT 1`, userID, kind)
}

func (s *Store) oneDocument(ctx context.Context, where string, args ...any) (Document, error) {
	var d Document
	err := s.queryRow(ctx, `SELECT id, user_id, kind, filename, content_type, body, created_at FROM documents `+where, args...).
		Scan(&d.ID, &d.UserID, &d.Kind, &d.Filename, &d.ContentType, &d.Body, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return Document{}, domain.NotFound("document")
	}
	return d, err
}

func (s *Store) DeleteDocument(ctx context.Context, userID, id string) error {
	res, err := s.exec(ctx, `DELETE FROM documents WHERE user_id=? AND id=?`, userID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.NotFound("document")
	}
	return nil
}
