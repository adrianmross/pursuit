package service

import (
	"context"
	"strings"

	"github.com/pursuit/pursuit/internal/domain"
)

type NoteInput struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

// NotePatch edits a notes file. body replaces it and needs version; append
// and set_question edit part of it, as on a round.
type NotePatch struct {
	Version     int                   `json:"version"`
	Name        *string               `json:"name"`
	Body        *string               `json:"body"`
	SetQuestion *domain.RoundQuestion `json:"set_question"`
	Append      *string               `json:"append"`
}

func noteName(raw string) (string, error) {
	name, err := domain.BoundText("name", strings.Join(strings.Fields(raw), " "), 100)
	if err == nil && name == "" {
		err = domain.Validation("name is required").WithHint("name the file, e.g. salary research")
	}
	return name, err
}

func (s *Service) AddNote(ctx context.Context, appID string, in NoteInput) (domain.Note, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Note{}, err
	}
	name, err := noteName(in.Name)
	if err != nil {
		return domain.Note{}, err
	}
	body, err := domain.BoundText("body", in.Body, notesMax)
	if err != nil {
		return domain.Note{}, err
	}
	now := s.now()
	n := domain.Note{ID: s.NewID(), UserID: a.UserID, ApplicationID: appID, Name: name, Body: body, Questions: domain.ParseRoundQuestions(body), Version: 1, CreatedAt: now, UpdatedAt: now}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		app, err := s.DB.GetApplication(ctx, a.UserID, appID)
		if err != nil {
			return err
		}
		if err := s.DB.CreateNote(ctx, n); err != nil {
			return err
		}
		return s.audit(ctx, a, "note.create", "note", n.ID, n.Name+" ("+app.Title+")", map[string]any{"application_id": appID})
	})
	return n, err
}

func (s *Service) ListNotes(ctx context.Context, appID string) ([]domain.Note, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	if _, err := s.DB.GetApplication(ctx, a.UserID, appID); err != nil {
		return nil, err
	}
	return s.DB.ListNotes(ctx, a.UserID, appID)
}

func (s *Service) GetNote(ctx context.Context, id string) (domain.Note, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return domain.Note{}, err
	}
	return s.DB.GetNote(ctx, a.UserID, id)
}

func (s *Service) UpdateNote(ctx context.Context, id string, p NotePatch) (domain.Note, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Note{}, err
	}
	if p.Body != nil && p.Version < 1 {
		return domain.Note{}, versionRequired("note").WithHint("body replaces the whole file: pass version, or use append or set_question")
	}
	var out domain.Note
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		cur, err := s.DB.GetNote(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if p.Version > 0 && cur.Version != p.Version {
			return domain.VersionConflict("note", cur.Version)
		}
		next := cur
		if p.Name != nil {
			if next.Name, err = noteName(*p.Name); err != nil {
				return err
			}
		}
		if p.Body != nil {
			next.Body = *p.Body
		}
		if q := p.SetQuestion; q != nil {
			prompt, err := domain.BoundText("set_question.prompt", strings.Join(strings.Fields(q.Prompt), " "), 500)
			if err != nil {
				return err
			}
			if prompt == "" {
				return domain.Validation("set_question needs a prompt")
			}
			next.Body = domain.SetRoundQuestion(next.Body, prompt, q.Answer)
		}
		if p.Append != nil && strings.TrimSpace(*p.Append) != "" {
			next.Body = domain.AppendNotes(next.Body, *p.Append)
		}
		if next.Body, err = domain.BoundText("body", next.Body, notesMax); err != nil {
			return err
		}
		next.UpdatedAt = s.now()
		if err := s.DB.UpdateNote(ctx, next); err != nil {
			return err
		}
		if err := s.audit(ctx, a, "note.update", "note", id, next.Name, map[string]any{"application_id": cur.ApplicationID}); err != nil {
			return err
		}
		out, err = s.DB.GetNote(ctx, a.UserID, id)
		return err
	})
	return out, err
}

func (s *Service) DeleteNote(ctx context.Context, id string, version int) (domain.Deleted, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Deleted{}, err
	}
	if version < 1 {
		return domain.Deleted{}, versionRequired("note")
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.DeleteNote(ctx, a.UserID, id, version); err != nil {
			return err
		}
		return s.audit(ctx, a, "note.delete", "note", id, "deleted note", nil)
	})
	if err != nil {
		return domain.Deleted{}, err
	}
	return domain.Deleted{ID: id, Deleted: true}, nil
}
