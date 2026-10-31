package sqlstore

import (
	"context"
	"database/sql"

	"github.com/pursuit/pursuit/internal/domain"
)

const taskCols = `id, user_id, application_id, stage, title, notes, due_at, done_at, version, created_at, updated_at`

func scanTask(row scanner) (domain.Task, error) {
	var t domain.Task
	var due, done, created, updated string
	if err := row.Scan(&t.ID, &t.UserID, &t.ApplicationID, &t.Stage, &t.Title, &t.Notes, &due, &done, &t.Version, &created, &updated); err != nil {
		return domain.Task{}, err
	}
	var err error
	if t.DueAt, err = parseTimeText(due); err != nil {
		return domain.Task{}, err
	}
	if t.DoneAt, err = parseTimeText(done); err != nil {
		return domain.Task{}, err
	}
	if t.CreatedAt, err = parseTime(created); err != nil {
		return domain.Task{}, err
	}
	t.UpdatedAt, err = parseTime(updated)
	return t, err
}

func (s *Store) CreateTask(ctx context.Context, t domain.Task) error {
	_, err := s.exec(ctx, `INSERT INTO tasks (`+taskCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.UserID, t.ApplicationID, t.Stage, t.Title, t.Notes, timeText(t.DueAt), timeText(t.DoneAt), t.Version, formatTime(t.CreatedAt), formatTime(t.UpdatedAt))
	return mapWriteErr(err)
}

func (s *Store) GetTask(ctx context.Context, userID, id string) (domain.Task, error) {
	t, err := scanTask(s.queryRow(ctx, `SELECT `+taskCols+` FROM tasks WHERE user_id=? AND id=?`, userID, id))
	if err == sql.ErrNoRows {
		return domain.Task{}, domain.NotFound("task")
	}
	return t, err
}

// ListTasks returns an application's tasks in the order they were added.
func (s *Store) ListTasks(ctx context.Context, userID, appID string) ([]domain.Task, error) {
	rows, err := s.query(ctx, `SELECT `+taskCols+` FROM tasks WHERE user_id=? AND application_id=? ORDER BY created_at, id`, userID, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) taskVersion(ctx context.Context, userID, id string) func() (int, error) {
	return func() (int, error) {
		cur, err := s.GetTask(ctx, userID, id)
		if err != nil {
			return 0, err
		}
		return cur.Version, nil
	}
}

func (s *Store) UpdateTask(ctx context.Context, t domain.Task) error {
	res, err := s.exec(ctx, `UPDATE tasks SET stage=?, title=?, notes=?, due_at=?, done_at=?, version=version+1, updated_at=? WHERE id=? AND user_id=? AND version=?`,
		t.Stage, t.Title, t.Notes, timeText(t.DueAt), timeText(t.DoneAt), formatTime(t.UpdatedAt), t.ID, t.UserID, t.Version)
	if err != nil {
		return mapWriteErr(err)
	}
	return changed(res, s.taskVersion(ctx, t.UserID, t.ID), "task")
}

func (s *Store) DeleteTask(ctx context.Context, userID, id string, version int) error {
	res, err := s.exec(ctx, `DELETE FROM tasks WHERE id=? AND user_id=? AND version=?`, id, userID, version)
	if err != nil {
		return err
	}
	return changed(res, s.taskVersion(ctx, userID, id), "task")
}
