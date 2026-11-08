package service

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

const maxAvatar = 256 << 10

type MePatch struct {
	Name   *string `json:"name"`
	Avatar *string `json:"avatar"`
	Email  *string `json:"email"`
	// AnnouncementsSeen only moves forward: an older time than the stored
	// one is ignored, so a stale tab can't bring a dismissed card back.
	AnnouncementsSeen *string `json:"announcements_seen"`
}

// UpdateMe changes the display name and avatar. Email is the identity key
// for sign-in, so only the PURSUIT_TRUST_PROXY_AUTH owner may change it: the
// platform in front authenticates them, not the email, and they are the only
// user, so nothing else can be keyed on the old address.
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
	if in.Email != nil {
		if !a.Proxy {
			return domain.User{}, domain.Validation("email is your sign-in and can't be changed here").WithHint("change it through your sign-in provider")
		}
		v, err := domain.ParseEmail(*in.Email)
		if err != nil {
			return domain.User{}, err
		}
		if v == "" {
			return domain.User{}, domain.Validation("email is required")
		}
		user.Email = v
	}
	if in.AnnouncementsSeen != nil {
		t, err := time.Parse(time.RFC3339, *in.AnnouncementsSeen)
		if err != nil {
			return domain.User{}, domain.Validation("announcements_seen must be an RFC 3339 time").WithHint("2026-11-07T00:00:00Z")
		}
		if v := t.UTC().Format(time.RFC3339); v > user.AnnouncementsSeen {
			user.AnnouncementsSeen = v
		}
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

// DocumentView is a document without its body.
type DocumentView struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Label       string `json:"label"`
	// Primary is the main résumé or cover letter: the one prepare attaches.
	Primary   bool   `json:"primary"`
	Size      int    `json:"size"`
	Storage   string `json:"storage"`
	CreatedAt string `json:"created_at"`
}

// kinds with one main document each.
var primaryKinds = map[string]bool{"base_resume": true, "cover_letter": true}

func documentKind(kind string) (string, error) {
	switch k := strings.ToLower(strings.TrimSpace(kind)); k {
	case "base_resume", "cover_letter", "other":
		return k, nil
	default:
		return "", domain.Validation("invalid document kind %q", kind).WithHint("kind is base_resume, cover_letter, or other")
	}
}

// stored is how a file sits in a body column: text as is, binary as base64.
func stored(contentType string, raw []byte) string {
	if isText(contentType) {
		return string(raw)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func docKey(userID, id string) string { return "documents/" + userID + "/" + id }

// withBody fills d.Body from the bucket when the file lives there.
func (s *Service) withBody(ctx context.Context, d sqlstore.Document) (sqlstore.Document, error) {
	if d.Storage != "s3" {
		return d, nil
	}
	if s.Blobs == nil {
		return d, domain.Conflict("this document is in file storage, which isn't configured", "set PURSUIT_S3_BUCKET and its credentials again")
	}
	raw, err := s.Blobs.Get(ctx, d.ObjectKey)
	if err != nil {
		return d, err
	}
	d.Body = stored(d.ContentType, raw)
	return d, nil
}

func (s *Service) AddDocument(ctx context.Context, kind, filename, contentType, label string, body []byte) (DocumentView, error) {
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
	label = strings.TrimSpace(label)
	if len(label) > 120 {
		return DocumentView{}, domain.Validation("label is too long").WithHint("keep it under 120 characters")
	}
	d := sqlstore.Document{
		ID: s.NewID(), UserID: a.UserID, Kind: kind, Filename: filename, ContentType: contentType,
		Label: label, Size: len(body), Storage: "db", CreatedAt: domain.FormatTime(s.now()),
	}
	if primaryKinds[kind] {
		n, err := s.DB.CountDocuments(ctx, a.UserID, kind)
		if err != nil {
			return DocumentView{}, err
		}
		// The first of a kind is the main one; later ones are variants until chosen.
		d.Primary = n == 0
	}
	if s.Blobs != nil {
		d.Storage, d.ObjectKey = "s3", docKey(a.UserID, d.ID)
		if err := s.Blobs.Put(ctx, d.ObjectKey, contentType, body); err != nil {
			return DocumentView{}, err
		}
	} else {
		d.Body = stored(contentType, body)
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.CreateDocument(ctx, d); err != nil {
			return err
		}
		return s.audit(ctx, a, "document.create", "document", d.ID, kind, map[string]any{"storage": d.Storage, "primary": d.Primary})
	})
	if err != nil && d.Storage == "s3" {
		_ = s.Blobs.Delete(ctx, d.ObjectKey)
	}
	if err != nil {
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

// GetDocument returns the document with its body, wherever it's stored.
func (s *Service) GetDocument(ctx context.Context, id string) (sqlstore.Document, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return sqlstore.Document{}, err
	}
	d, err := s.DB.GetDocument(ctx, a.UserID, id)
	if err != nil {
		return d, err
	}
	return s.withBody(ctx, d)
}

// DocumentPatch renames a document, makes it the main one of its kind, or
// replaces a text document's content (a cover letter written in place).
type DocumentPatch struct {
	Label   *string `json:"label"`
	Primary *bool   `json:"primary"`
	Body    *string `json:"body"`
}

func (s *Service) UpdateDocument(ctx context.Context, id string, in DocumentPatch) (DocumentView, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return DocumentView{}, err
	}
	d, err := s.DB.GetDocument(ctx, a.UserID, id)
	if err != nil {
		return DocumentView{}, err
	}
	if in.Label != nil {
		d.Label = strings.TrimSpace(*in.Label)
		if len(d.Label) > 120 {
			return DocumentView{}, domain.Validation("label is too long").WithHint("keep it under 120 characters")
		}
	}
	if in.Primary != nil && !*in.Primary {
		return DocumentView{}, domain.Validation("make another document the main one instead").WithHint(`{"primary": true} on the one you want`)
	}
	if in.Primary != nil && !primaryKinds[d.Kind] {
		return DocumentView{}, domain.Validation("only a résumé or cover letter has a main version")
	}
	if in.Body != nil {
		if !isText(d.ContentType) {
			return DocumentView{}, domain.Validation("only a text document can be edited in place").WithHint("upload the new file instead")
		}
		if len(*in.Body) == 0 || len(*in.Body) > maxArtifact {
			return DocumentView{}, domain.Validation("body must be 1 byte to 5MB")
		}
		d.Size = len(*in.Body)
		if d.Storage == "s3" {
			if s.Blobs == nil {
				return DocumentView{}, domain.Conflict("this document is in file storage, which isn't configured", "set PURSUIT_S3_BUCKET again")
			}
			if err := s.Blobs.Put(ctx, d.ObjectKey, d.ContentType, []byte(*in.Body)); err != nil {
				return DocumentView{}, err
			}
		} else {
			d.Body = *in.Body
		}
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.UpdateDocument(ctx, d); err != nil {
			return err
		}
		if in.Primary != nil {
			if err := s.DB.SetPrimaryDocument(ctx, a.UserID, d.Kind, d.ID); err != nil {
				return err
			}
			d.Primary = true
		}
		return s.audit(ctx, a, "document.update", "document", d.ID, "updated document", map[string]any{
			"label": in.Label != nil, "primary": in.Primary != nil, "body": in.Body != nil,
		})
	})
	if err != nil {
		return DocumentView{}, err
	}
	return docView(d), nil
}

func (s *Service) DeleteDocument(ctx context.Context, id string) (domain.Deleted, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Deleted{}, err
	}
	d, err := s.DB.GetDocument(ctx, a.UserID, id)
	if err != nil {
		return domain.Deleted{}, err
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.DeleteDocument(ctx, a.UserID, id); err != nil {
			return err
		}
		// Deleting the main one hands the role to the newest that's left.
		if d.Primary {
			next, err := s.DB.PrimaryDocument(ctx, a.UserID, d.Kind)
			if err == nil {
				if err := s.DB.SetPrimaryDocument(ctx, a.UserID, d.Kind, next.ID); err != nil {
					return err
				}
			} else if !domain.IsCode(err, "not_found") {
				return err
			}
		}
		return s.audit(ctx, a, "document.delete", "document", id, "deleted document", nil)
	})
	if err != nil {
		return domain.Deleted{}, err
	}
	if d.Storage == "s3" && s.Blobs != nil {
		// ponytail: a failed delete leaves an orphan object; a bucket lifecycle rule or sweep can collect them.
		_ = s.Blobs.Delete(ctx, d.ObjectKey)
	}
	return domain.Deleted{ID: id, Deleted: true}, nil
}

// primaryDocument is the main document of kind with its body, for prepare.
func (s *Service) primaryDocument(ctx context.Context, userID, kind string) (sqlstore.Document, error) {
	d, err := s.DB.PrimaryDocument(ctx, userID, kind)
	if err != nil {
		return d, err
	}
	return s.withBody(ctx, d)
}

func docView(d sqlstore.Document) DocumentView {
	return DocumentView{ID: d.ID, Kind: d.Kind, Filename: d.Filename, ContentType: d.ContentType, Label: d.Label,
		Primary: d.Primary, Size: d.Size, Storage: d.Storage, CreatedAt: d.CreatedAt}
}
