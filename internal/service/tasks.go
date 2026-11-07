package service

import (
	"context"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
)

type TaskInput struct {
	Stage string `json:"stage"`
	// RoundID puts the task on one round; its stage then comes from the round.
	RoundID string `json:"round_id"`
	Title string `json:"title"`
	Notes string `json:"notes"`
	DueAt string `json:"due_at"`
	Done  bool   `json:"done"`
}

type TaskPatch struct {
	Version int     `json:"version"`
	Stage   *string `json:"stage"`
	RoundID *string `json:"round_id"`
	Title   *string `json:"title"`
	Notes   *string `json:"notes"`
	DueAt   *string `json:"due_at"`
	Done    *bool   `json:"done"`
}

func taskTitle(raw string) (string, error) {
	title, err := domain.BoundText("title", raw, 300)
	if err == nil && title == "" {
		err = domain.Validation("title is required").WithHint("pass what needs doing as title")
	}
	return title, err
}

func (s *Service) parseDue(raw string) (*time.Time, error) {
	t, err := s.parseWhen(raw)
	if err != nil {
		return nil, domain.Validation("invalid due_at").WithHint("use RFC3339 or YYYY-MM-DD")
	}
	return t, nil
}

// AddTask records a to-do on an application. With no stage it joins the
// application's current stage, or to_apply before it reaches one.
func (s *Service) AddTask(ctx context.Context, appID string, in TaskInput) (domain.Task, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Task{}, err
	}
	title, err := taskTitle(in.Title)
	if err != nil {
		return domain.Task{}, err
	}
	notes, err := domain.BoundText("notes", in.Notes, 50000)
	if err != nil {
		return domain.Task{}, err
	}
	due, err := s.parseDue(in.DueAt)
	if err != nil {
		return domain.Task{}, err
	}
	now := s.now()
	t := domain.Task{
		ID: s.NewID(), UserID: a.UserID, ApplicationID: appID, Title: title, Notes: notes,
		DueAt: due, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if in.Done {
		t.DoneAt = &now
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		app, err := s.DB.GetApplication(ctx, a.UserID, appID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(in.Stage) != "" {
			if t.Stage, err = domain.ParseTaskStage(in.Stage); err != nil {
				return err
			}
		} else {
			t.Stage = domain.TaskStageKey(app.Stage)
		}
		if err := s.onRound(ctx, a.UserID, appID, strings.TrimSpace(in.RoundID), &t); err != nil {
			return err
		}
		if err := s.DB.CreateTask(ctx, t); err != nil {
			return err
		}
		return s.audit(ctx, a, "task.create", "task", t.ID, t.Title+" ("+app.Title+")", map[string]any{"application_id": appID, "stage": t.Stage})
	})
	return t, err
}

// onRound ties t to a round of the same application, taking the round's stage.
// An empty id unties it, keeping the stage.
func (s *Service) onRound(ctx context.Context, userID, appID, roundID string, t *domain.Task) error {
	if roundID == "" {
		t.RoundID = ""
		return nil
	}
	r, err := s.DB.GetInterview(ctx, userID, roundID)
	if err != nil {
		return err
	}
	if r.ApplicationID != appID {
		return domain.Validation("round %s is on another application", roundID)
	}
	t.RoundID, t.Stage = r.ID, r.Stage
	return nil
}

func (s *Service) ListTasks(ctx context.Context, appID string) ([]domain.Task, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	if _, err := s.DB.GetApplication(ctx, a.UserID, appID); err != nil {
		return nil, err
	}
	return s.DB.ListTasks(ctx, a.UserID, appID)
}

func (s *Service) GetTask(ctx context.Context, id string) (domain.Task, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return domain.Task{}, err
	}
	return s.DB.GetTask(ctx, a.UserID, id)
}

func (s *Service) UpdateTask(ctx context.Context, id string, p TaskPatch) (domain.Task, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Task{}, err
	}
	if p.Version < 1 {
		return domain.Task{}, versionRequired("task")
	}
	var out domain.Task
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		cur, err := s.DB.GetTask(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if cur.Version != p.Version {
			return domain.VersionConflict("task", cur.Version)
		}
		next := cur
		if p.Stage != nil {
			if next.Stage, err = domain.ParseTaskStage(*p.Stage); err != nil {
				return err
			}
		}
		if p.RoundID != nil {
			if err := s.onRound(ctx, a.UserID, cur.ApplicationID, strings.TrimSpace(*p.RoundID), &next); err != nil {
				return err
			}
		} else if p.Stage != nil && next.Stage != cur.Stage {
			// Moving to another stage leaves the round behind.
			next.RoundID = ""
		}
		if p.Title != nil {
			if next.Title, err = taskTitle(*p.Title); err != nil {
				return err
			}
		}
		if p.Notes != nil {
			if next.Notes, err = domain.BoundText("notes", *p.Notes, 50000); err != nil {
				return err
			}
		}
		if p.DueAt != nil {
			if next.DueAt, err = s.parseDue(*p.DueAt); err != nil {
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
		if err := s.DB.UpdateTask(ctx, next); err != nil {
			return err
		}
		if err := s.audit(ctx, a, "task.update", "task", id, next.Title, nil); err != nil {
			return err
		}
		out, err = s.DB.GetTask(ctx, a.UserID, id)
		return err
	})
	return out, err
}

func (s *Service) DeleteTask(ctx context.Context, id string, version int) (domain.Deleted, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Deleted{}, err
	}
	if version < 1 {
		return domain.Deleted{}, versionRequired("task")
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.DeleteTask(ctx, a.UserID, id, version); err != nil {
			return err
		}
		return s.audit(ctx, a, "task.delete", "task", id, "deleted task", nil)
	})
	if err != nil {
		return domain.Deleted{}, err
	}
	return domain.Deleted{ID: id, Deleted: true}, nil
}
