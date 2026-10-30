package service

import (
	"context"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
)

type InterviewInput struct {
	Stage       string `json:"stage"`
	Kind        string `json:"kind"`
	Interviewer string `json:"interviewer"`
	ScheduledAt string `json:"scheduled_at"`
	Prep        string `json:"prep"`
	Notes       string `json:"notes"`
	Done        bool   `json:"done"`
}

type InterviewPatch struct {
	Version     int     `json:"version"`
	Stage       *string `json:"stage"`
	Kind        *string `json:"kind"`
	Interviewer *string `json:"interviewer"`
	ScheduledAt *string `json:"scheduled_at"`
	Prep        *string `json:"prep"`
	Notes       *string `json:"notes"`
	Done        *bool   `json:"done"`
}

func (s *Service) parseWhen(raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	t, err := domain.ParseTime(raw)
	if err != nil {
		return nil, domain.Validation("invalid scheduled_at").WithHint("use RFC3339 or YYYY-MM-DD")
	}
	return &t, nil
}

// AddInterview records a round for an application. Any stage can have rounds.
// With no stage it joins the application's current stage, or prepared when it
// hasn't reached one.
func (s *Service) AddInterview(ctx context.Context, appID string, in InterviewInput) (domain.Interview, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Interview{}, err
	}
	kind, err := domain.BoundText("kind", in.Kind, 100)
	if err != nil {
		return domain.Interview{}, err
	}
	who, err := domain.BoundText("interviewer", in.Interviewer, 300)
	if err != nil {
		return domain.Interview{}, err
	}
	prep, err := domain.BoundText("prep", in.Prep, 50000)
	if err != nil {
		return domain.Interview{}, err
	}
	notes, err := domain.BoundText("notes", in.Notes, 50000)
	if err != nil {
		return domain.Interview{}, err
	}
	when, err := s.parseWhen(in.ScheduledAt)
	if err != nil {
		return domain.Interview{}, err
	}
	now := s.now()
	iv := domain.Interview{
		ID: s.NewID(), UserID: a.UserID, ApplicationID: appID, Kind: kind, Interviewer: who,
		ScheduledAt: when, Prep: prep, Notes: notes, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if in.Done {
		iv.DoneAt = &now
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		app, err := s.DB.GetApplication(ctx, a.UserID, appID)
		if err != nil {
			return err
		}
		switch {
		case strings.TrimSpace(in.Stage) != "":
			if iv.Stage, err = domain.ParseInterviewStage(in.Stage); err != nil {
				return err
			}
		default:
			iv.Stage = domain.StageKey(max(app.Stage, 1))
		}
		if err := s.DB.CreateInterview(ctx, iv); err != nil {
			return err
		}
		return s.audit(ctx, a, "interview.create", "interview", iv.ID, iv.Stage+" "+app.Title, map[string]any{"application_id": appID})
	})
	return iv, err
}

func (s *Service) ListInterviews(ctx context.Context, appID string) ([]domain.Interview, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	if _, err := s.DB.GetApplication(ctx, a.UserID, appID); err != nil {
		return nil, err
	}
	return s.DB.ListInterviews(ctx, a.UserID, appID)
}

func (s *Service) GetInterview(ctx context.Context, id string) (domain.Interview, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return domain.Interview{}, err
	}
	return s.DB.GetInterview(ctx, a.UserID, id)
}

func (s *Service) UpdateInterview(ctx context.Context, id string, p InterviewPatch) (domain.Interview, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Interview{}, err
	}
	if p.Version < 1 {
		return domain.Interview{}, versionRequired("interview")
	}
	var out domain.Interview
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		cur, err := s.DB.GetInterview(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if cur.Version != p.Version {
			return domain.VersionConflict("interview", cur.Version)
		}
		next := cur
		if p.Stage != nil {
			if next.Stage, err = domain.ParseInterviewStage(*p.Stage); err != nil {
				return err
			}
		}
		for _, f := range []struct {
			name string
			in   *string
			dst  *string
			max  int
		}{
			{"kind", p.Kind, &next.Kind, 100},
			{"interviewer", p.Interviewer, &next.Interviewer, 300},
			{"prep", p.Prep, &next.Prep, 50000},
			{"notes", p.Notes, &next.Notes, 50000},
		} {
			if f.in == nil {
				continue
			}
			if *f.dst, err = domain.BoundText(f.name, *f.in, f.max); err != nil {
				return err
			}
		}
		if p.ScheduledAt != nil {
			if next.ScheduledAt, err = s.parseWhen(*p.ScheduledAt); err != nil {
				return err
			}
		}
		now := s.now()
		if p.Done != nil {
			switch {
			case *p.Done && cur.DoneAt == nil:
				next.DoneAt = &now
			case !*p.Done:
				next.DoneAt = nil
			}
		}
		next.UpdatedAt = now
		if err := s.DB.UpdateInterview(ctx, next); err != nil {
			return err
		}
		if err := s.audit(ctx, a, "interview.update", "interview", id, next.Stage+" "+next.Kind, nil); err != nil {
			return err
		}
		out, err = s.DB.GetInterview(ctx, a.UserID, id)
		return err
	})
	return out, err
}

func (s *Service) DeleteInterview(ctx context.Context, id string, version int) (domain.Deleted, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Deleted{}, err
	}
	if version < 1 {
		return domain.Deleted{}, versionRequired("interview")
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.DeleteInterview(ctx, a.UserID, id, version); err != nil {
			return err
		}
		return s.audit(ctx, a, "interview.delete", "interview", id, "deleted interview", nil)
	})
	if err != nil {
		return domain.Deleted{}, err
	}
	return domain.Deleted{ID: id, Deleted: true}, nil
}
