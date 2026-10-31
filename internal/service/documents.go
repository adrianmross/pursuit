package service

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

const maxAvatar = 256 << 10

type MePatch struct {
	Name   *string `json:"name"`
	Avatar *string `json:"avatar"`
}

// UpdateMe changes the display name and avatar. Email is the identity key
// for sign-in, so it is not editable here.
func (s *Service) UpdateMe(ctx context.Context, in MePatch) (domain.User, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.User{}, err
	}
	user, err := s.DB.GetUser(ctx, a.UserID)
	if err != nil {
		return domain.User{}, err
	}
	if in.Name != nil {
		if user.Name, err = domain.RequireText("name", *in.Name, 200); err != nil {
			return domain.User{}, err
		}
	}
	if in.Avatar != nil {
		v := strings.TrimSpace(*in.Avatar)
		if v != "" && !validAvatar(v) {
			return domain.User{}, domain.Validation("avatar must be a png, jpeg, or webp data URL").WithHint("data:image/png;base64,...")
		}
		if len(v) > maxAvatar {
			return domain.User{}, domain.Validation("avatar is too large").WithHint("keep it under 256KB; resize to about 256px")
		}
		user.Avatar = v
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.UpdateUserProfile(ctx, user); err != nil {
			return err
		}
		return s.audit(ctx, a, "user.update", "user", user.ID, "updated profile", nil)
	})
	return user, err
}

// validAvatar admits only raster data URLs. SVG is excluded: it can carry
// script and the dashboard renders this in an <img> on its own origin.
func validAvatar(v string) bool {
	for _, p := range []string{"data:image/png;base64,", "data:image/jpeg;base64,", "data:image/webp;base64,"} {
		if rest, ok := strings.CutPrefix(v, p); ok {
			_, err := base64.StdEncoding.DecodeString(rest)
			return err == nil
		}
	}
	return false
}

type DocumentView struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	CreatedAt   string `json:"created_at"`
}

func documentKind(kind string) (string, error) {
	switch k := strings.ToLower(strings.TrimSpace(kind)); k {
	case "base_resume", "cover_letter", "other":
		return k, nil
	default:
		return "", domain.Validation("invalid document kind %q", kind).WithHint("kind is base_resume, cover_letter, or other")
	}
}

func (s *Service) AddDocument(ctx context.Context, kind, filename, contentType string, body []byte) (DocumentView, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return DocumentView{}, err
	}
	if kind, err = documentKind(kind); err != nil {
		return DocumentView{}, err
	}
	if len(body) == 0 {
		return DocumentView{}, domain.Validation("document body is empty").WithHint("pass a file")
	}
	if len(body) > maxArtifact {
		return DocumentView{}, domain.Validation("document is too large").WithHint("keep documents under 5MB")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	stored := string(body)
	if !isText(contentType) {
		stored = base64.StdEncoding.EncodeToString(body)
	}
	d := sqlstore.Document{
		ID: s.NewID(), UserID: a.UserID, Kind: kind, Filename: filename,
		ContentType: contentType, Body: stored, CreatedAt: domain.FormatTime(s.now()),
	}
	if err := s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.CreateDocument(ctx, d); err != nil {
			return err
		}
		return s.audit(ctx, a, "document.create", "document", d.ID, kind, nil)
	}); err != nil {
		return DocumentView{}, err
	}
	return docView(d), nil
}

func (s *Service) ListDocuments(ctx context.Context) ([]DocumentView, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.ListDocuments(ctx, a.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]DocumentView, len(rows))
	for i, d := range rows {
		out[i] = docView(d)
	}
	return out, nil
}

func (s *Service) GetDocument(ctx context.Context, id string) (sqlstore.Document, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return sqlstore.Document{}, err
	}
	return s.DB.GetDocument(ctx, a.UserID, id)
}

func (s *Service) DeleteDocument(ctx context.Context, id string) (domain.Deleted, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Deleted{}, err
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.DeleteDocument(ctx, a.UserID, id); err != nil {
			return err
		}
		return s.audit(ctx, a, "document.delete", "document", id, "deleted document", nil)
	})
	if err != nil {
		return domain.Deleted{}, err
	}
	return domain.Deleted{ID: id, Deleted: true}, nil
}

func docView(d sqlstore.Document) DocumentView {
	return DocumentView{ID: d.ID, Kind: d.Kind, Filename: d.Filename, ContentType: d.ContentType, CreatedAt: d.CreatedAt}
}
