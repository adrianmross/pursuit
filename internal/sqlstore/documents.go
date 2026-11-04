package sqlstore

import (
	"context"
	"database/sql"

	"github.com/pursuit/pursuit/internal/domain"
)

// Document is a user-level file: a résumé, a cover letter, a LinkedIn PDF
// export. Body is base64 for binary content, as with Artifact, and empty
// when Storage is "s3" (the file is at ObjectKey in the bucket).
type Document struct {
	ID          string
	UserID      string
	Kind        string
	Filename    string
	ContentType string
	Body        string
	CreatedAt   string
	Storage     string
	ObjectKey   string
	Size        int
	Label       string
	Primary     bool
}

const docCols = `id, user_id, kind, filename, content_type, created_at, storage, object_key, size, label, is_primary`

func scanDoc(sc interface{ Scan(...any) error }, d *Document, extra ...any) error {
	var primary int
	err := sc.Scan(append([]any{&d.ID, &d.UserID, &d.Kind, &d.Filename, &d.ContentType, &d.CreatedAt, &d.Storage, &d.ObjectKey, &d.Size, &d.Label, &primary}, extra...)...)
	d.Primary = primary != 0
	return err
}

func (s *Store) CreateDocument(ctx context.Context, d Document) error {
	if d.Storage == "" {
		d.Storage = "db"
	}
	primary := 0
	if d.Primary {
		primary = 1
	}
	_, err := s.exec(ctx, `INSERT INTO documents (`+docCols+`, body) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.UserID, d.Kind, d.Filename, d.ContentType, d.CreatedAt, d.Storage, d.ObjectKey, d.Size, d.Label, primary, d.Body)
	return err
}

// ListDocuments is every document without its body, main ones first in each kind.
func (s *Store) ListDocuments(ctx context.Context, userID string) ([]Document, error) {
	rows, err := s.query(ctx, `SELECT `+docCols+` FROM documents WHERE user_id=? ORDER BY kind, is_primary DESC, created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		var d Document
		if err := scanDoc(rows, &d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDocument(ctx context.Context, userID, id string) (Document, error) {
	return s.oneDocument(ctx, `WHERE user_id=? AND id=?`, userID, id)
}

// PrimaryDocument returns the main document of kind, else the newest, or not_found.
func (s *Store) PrimaryDocument(ctx context.Context, userID, kind string) (Document, error) {
	return s.oneDocument(ctx, `WHERE user_id=? AND kind=? ORDER BY is_primary DESC, created_at DESC, id DESC LIMIT 1`, userID, kind)
}

func (s *Store) CountDocuments(ctx context.Context, userID, kind string) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM documents WHERE user_id=? AND kind=?`, userID, kind).Scan(&n)
	return n, err
}

func (s *Store) oneDocument(ctx context.Context, where string, args ...any) (Document, error) {
	var d Document
	err := scanDoc(s.queryRow(ctx, `SELECT `+docCols+`, body FROM documents `+where, args...), &d, &d.Body)
	if err == sql.ErrNoRows {
		return Document{}, domain.NotFound("document")
	}
	return d, err
}

// UpdateDocument writes label, body and size. Primary is changed only by SetPrimaryDocument.
func (s *Store) UpdateDocument(ctx context.Context, d Document) error {
	_, err := s.exec(ctx, `UPDATE documents SET label=?, body=?, size=? WHERE user_id=? AND id=?`, d.Label, d.Body, d.Size, d.UserID, d.ID)
	return err
}

// SetPrimaryDocument makes id the only main document of its kind. Call inside WithTx.
func (s *Store) SetPrimaryDocument(ctx context.Context, userID, kind, id string) error {
	if _, err := s.exec(ctx, `UPDATE documents SET is_primary=0 WHERE user_id=? AND kind=?`, userID, kind); err != nil {
		return err
	}
	_, err := s.exec(ctx, `UPDATE documents SET is_primary=1 WHERE user_id=? AND id=?`, userID, id)
	return err
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
