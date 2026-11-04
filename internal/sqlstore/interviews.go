package sqlstore

import (
	"context"
	"database/sql"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
)

const interviewCols = `id, user_id, application_id, stage, kind, interviewer, scheduled_at, done_at, notes, in_notes, version, created_at, updated_at`

func timeText(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

func parseTimeText(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	t, err := parseTime(raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func scanInterview(row scanner) (domain.Interview, error) {
	var i domain.Interview
	var scheduled, done, created, updated string
	var inNotes int
	if err := row.Scan(&i.ID, &i.UserID, &i.ApplicationID, &i.Stage, &i.Kind, &i.Interviewer, &scheduled, &done, &i.Notes, &inNotes, &i.Version, &created, &updated); err != nil {
		return domain.Interview{}, err
	}
	i.InNotes = inNotes == 1
	var err error
	if i.ScheduledAt, err = parseTimeText(scheduled); err != nil {
		return domain.Interview{}, err
	}
	if i.DoneAt, err = parseTimeText(done); err != nil {
		return domain.Interview{}, err
	}
	if i.CreatedAt, err = parseTime(created); err != nil {
		return domain.Interview{}, err
	}
	i.Questions = domain.ParseRoundQuestions(i.Notes)
	i.UpdatedAt, err = parseTime(updated)
	return i, err
}

func (s *Store) CreateInterview(ctx context.Context, i domain.Interview) error {
	_, err := s.exec(ctx, `INSERT INTO interviews (`+interviewCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ID, i.UserID, i.ApplicationID, i.Stage, i.Kind, i.Interviewer, timeText(i.ScheduledAt), timeText(i.DoneAt), i.Notes, boolInt(i.InNotes), i.Version, formatTime(i.CreatedAt), formatTime(i.UpdatedAt))
	return mapWriteErr(err)
}

func (s *Store) GetInterview(ctx context.Context, userID, id string) (domain.Interview, error) {
	i, err := scanInterview(s.queryRow(ctx, `SELECT `+interviewCols+` FROM interviews WHERE user_id=? AND id=?`, userID, id))
	if err == sql.ErrNoRows {
		return domain.Interview{}, domain.NotFound("interview")
	}
	return i, err
}

// ListRoundQuestions reads every question out of the user's round notes,
// newest round first. ponytail: parses each document per call; store parsed
// questions in a table if this gets slow.
func (s *Store) ListRoundQuestions(ctx context.Context, userID string) ([]domain.BankQuestion, error) {
	rows, err := s.query(ctx, `SELECT i.id, i.application_id, i.stage, i.kind, i.notes, i.updated_at, r.title, c.name
		FROM interviews i JOIN applications a ON a.id = i.application_id JOIN roles r ON r.id = a.role_id JOIN companies c ON c.id = r.company_id
		WHERE i.user_id=? AND i.notes<>'' ORDER BY i.updated_at DESC, i.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.BankQuestion{}
	for rows.Next() {
		var b domain.BankQuestion
		var notes, updated string
		if err := rows.Scan(&b.RoundID, &b.ApplicationID, &b.Stage, &b.Kind, &notes, &updated, &b.Title, &b.Company); err != nil {
			return nil, err
		}
		if b.UpdatedAt, err = parseTime(updated); err != nil {
			return nil, err
		}
		for _, q := range domain.ParseRoundQuestions(notes) {
			b.RoundQuestion = q
			out = append(out, b)
		}
	}
	return out, rows.Err()
}

// ListInterviews returns an application's interviews, earliest scheduled
// first and unscheduled ones in the order they were added.
func (s *Store) ListInterviews(ctx context.Context, userID, appID string) ([]domain.Interview, error) {
	rows, err := s.query(ctx, `SELECT `+interviewCols+` FROM interviews WHERE user_id=? AND application_id=? ORDER BY created_at, id`, userID, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Interview{}
	for rows.Next() {
		i, err := scanInterview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *Store) UpdateInterview(ctx context.Context, i domain.Interview) error {
	res, err := s.exec(ctx, `UPDATE interviews SET stage=?, kind=?, interviewer=?, scheduled_at=?, done_at=?, notes=?, in_notes=?, version=version+1, updated_at=? WHERE id=? AND user_id=? AND version=?`,
		i.Stage, i.Kind, i.Interviewer, timeText(i.ScheduledAt), timeText(i.DoneAt), i.Notes, boolInt(i.InNotes), formatTime(i.UpdatedAt), i.ID, i.UserID, i.Version)
	if err != nil {
		return mapWriteErr(err)
	}
	return changed(res, func() (int, error) {
		cur, err := s.GetInterview(ctx, i.UserID, i.ID)
		if err != nil {
			return 0, err
		}
		return cur.Version, nil
	}, "interview")
}

func (s *Store) DeleteInterview(ctx context.Context, userID, id string, version int) error {
	res, err := s.exec(ctx, `DELETE FROM interviews WHERE id=? AND user_id=? AND version=?`, id, userID, version)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		// Its tasks stay on the stage.
		if _, err := s.exec(ctx, `UPDATE tasks SET round_id='' WHERE user_id=? AND round_id=?`, userID, id); err != nil {
			return err
		}
	}
	return changed(res, func() (int, error) {
		cur, err := s.GetInterview(ctx, userID, id)
		if err != nil {
			return 0, err
		}
		return cur.Version, nil
	}, "interview")
}
