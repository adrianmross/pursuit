package service

import (
	"cmp"
	"context"
	"encoding/base64"
	"errors"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/sqlstore"
	"github.com/pursuit/pursuit/internal/textract"
)

// artifactKindFor is the application file a document kind becomes.
var artifactKindFor = map[string]string{"base_resume": "resume_pdf", "cover_letter": "cover_letter"}

// UseDocument puts one of the user's documents on an application as its
// résumé or cover letter: the newest of a kind is the one in use. The main
// document is free; another version needs tailored_documents.
func (s *Service) UseDocument(ctx context.Context, appID, documentID string) (ArtifactView, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return ArtifactView{}, err
	}
	if _, err := s.DB.GetApplication(ctx, a.UserID, appID); err != nil {
		return ArtifactView{}, err
	}
	doc, err := s.DB.GetDocument(ctx, a.UserID, documentID)
	if err != nil {
		return ArtifactView{}, err
	}
	kind, ok := artifactKindFor[doc.Kind]
	if !ok {
		return ArtifactView{}, domain.Validation("only a résumé or cover letter can be used on an application")
	}
	if !doc.Primary {
		if err := s.RequireFeature(ctx, "tailored_documents"); err != nil {
			return ArtifactView{}, err
		}
	}
	if doc, err = s.withBody(ctx, doc); err != nil {
		return ArtifactView{}, err
	}
	body := doc.Body
	if kind == "resume_pdf" && isText(doc.ContentType) {
		// resume_pdf artifacts are always base64, whatever the source.
		body = base64.StdEncoding.EncodeToString([]byte(body))
	}
	now := domain.FormatTime(s.now())
	art := sqlstore.Artifact{
		ID: s.NewID(), UserID: a.UserID, ApplicationID: appID, Kind: kind, Filename: doc.Filename,
		ContentType: doc.ContentType, Body: body, CreatedAt: now, SourceDocumentID: doc.ID,
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.CreateArtifact(ctx, art); err != nil {
			return err
		}
		return s.audit(ctx, a, "artifact.create", "application", appID, kind+" from "+cmp.Or(doc.Label, doc.Filename), map[string]any{"artifact_id": art.ID, "document_id": doc.ID})
	})
	if err != nil {
		return ArtifactView{}, err
	}
	return ArtifactView{ID: art.ID, ApplicationID: appID, Kind: kind, Filename: art.Filename, ContentType: art.ContentType, CreatedAt: now, SourceDocumentID: doc.ID}, nil
}

// Side is one file in a comparison, as text.
type Side struct {
	Title string `json:"title"`
	Text  string `json:"text"`
	// DocumentID or ArtifactID says which file it is.
	DocumentID string `json:"document_id,omitempty"`
	ArtifactID string `json:"artifact_id,omitempty"`
	// Source is where the application's file came from: "default" (the main
	// document), "version" (another document) or "upload" (tailored here).
	Source string `json:"source,omitempty"`
}

type Comparison struct {
	Kind    string `json:"kind"`
	Default Side   `json:"default"`
	Current Side   `json:"current"`
	Same    bool   `json:"same"`
}

// rawStored undoes stored: a body column back to the file's bytes.
func rawStored(body, contentType string, alwaysBase64 bool) []byte {
	if alwaysBase64 || !isText(contentType) {
		if b, err := base64.StdEncoding.DecodeString(body); err == nil {
			return b
		}
	}
	return []byte(body)
}

// Compare puts the application's résumé or cover letter beside the main one, as text.
func (s *Service) Compare(ctx context.Context, appID, kind string) (Comparison, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return Comparison{}, err
	}
	if err := s.RequireFeature(ctx, "tailored_documents"); err != nil {
		return Comparison{}, err
	}
	docKind := map[string]string{"resume_pdf": "base_resume", "cover_letter": "cover_letter"}[kind]
	if docKind == "" {
		return Comparison{}, domain.Validation("kind is resume_pdf or cover_letter")
	}
	arts, err := s.DB.ListArtifacts(ctx, a.UserID, appID)
	if err != nil {
		return Comparison{}, err
	}
	var current *sqlstore.Artifact
	for i := range arts {
		if arts[i].Kind == kind {
			current = &arts[i] // oldest first, so the last is in use
		}
	}
	if current == nil {
		return Comparison{}, domain.NotFound(map[string]string{"resume_pdf": "résumé", "cover_letter": "cover letter"}[kind] + " on this application")
	}
	main, err := s.primaryDocument(ctx, a.UserID, docKind)
	if err != nil {
		return Comparison{}, err
	}
	full, err := s.DB.GetArtifact(ctx, a.UserID, current.ID)
	if err != nil {
		return Comparison{}, err
	}
	out := Comparison{Kind: kind,
		Default: Side{Title: cmp.Or(main.Label, main.Filename), DocumentID: main.ID},
		Current: Side{Title: full.Filename, ArtifactID: full.ID, Source: "upload"},
	}
	switch full.SourceDocumentID {
	case "":
	case main.ID:
		out.Current.Source = "default"
	default:
		out.Current.Source = "version"
		if d, err := s.DB.GetDocument(ctx, a.UserID, full.SourceDocumentID); err == nil {
			out.Current.Title = cmp.Or(d.Label, d.Filename)
		}
	}
	if out.Default.Text, err = textract.Text(main.ContentType, main.Filename, rawStored(main.Body, main.ContentType, false)); err != nil {
		return Comparison{}, unreadable(err, out.Default.Title)
	}
	if out.Current.Text, err = textract.Text(full.ContentType, full.Filename, rawStored(full.Body, full.ContentType, kind == "resume_pdf")); err != nil {
		return Comparison{}, unreadable(err, out.Current.Title)
	}
	out.Same = out.Default.Text == out.Current.Text
	return out, nil
}

func unreadable(err error, name string) error {
	hint := "compare works on PDF, Word (.docx), Markdown and text"
	if errors.Is(err, textract.ErrUnsupported) {
		return domain.Validation("can't read the text of %s", name).WithHint(hint)
	}
	return domain.Validation("can't read the text of %s: %s", name, err.Error()).WithHint(hint)
}
