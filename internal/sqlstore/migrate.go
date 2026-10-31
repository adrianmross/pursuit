package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
)

// advisoryLockKey is the Postgres session lock that serializes migrations and bootstrap.
const advisoryLockKey int64 = 594132718233451

func Migrate(ctx context.Context, db *sql.DB, dialect string) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if dialect == "postgres" {
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
			return fmt.Errorf("migration lock: %w", err)
		}
		defer conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", advisoryLockKey)
		return applyMigrations(ctx, conn, dialect, true)
	}

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	if err := applyMigrations(ctx, conn, dialect, false); err != nil {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	return nil
}

type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func applyMigrations(ctx context.Context, conn *sql.Conn, dialect string, perTx bool) error {
	preamble := rebind(dialect, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	if _, err := conn.ExecContext(ctx, preamble); err != nil {
		return fmt.Errorf("schema_migrations: %w", err)
	}
	for _, m := range migrations {
		var n int
		q := rebind(dialect, `SELECT COUNT(*) FROM schema_migrations WHERE version=?`)
		if err := conn.QueryRowContext(ctx, q, m.version).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		run := func(exec querier) error {
			for _, stmt := range m.stmts {
				if _, err := exec.ExecContext(ctx, stmt); err != nil {
					return fmt.Errorf("migration %s: %w", m.version, err)
				}
			}
			ins := rebind(dialect, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`)
			if _, err := exec.ExecContext(ctx, ins, m.version, "1970-01-01T00:00:00.000000000Z"); err != nil {
				return err
			}
			return nil
		}
		if perTx {
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			if err := run(tx); err != nil {
				_ = tx.Rollback()
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			continue
		}
		if err := run(conn); err != nil {
			return err
		}
	}
	return nil
}

type migration struct {
	version string
	stmts   []string
}

var migrations = []migration{{
	version: "001",
	stmts: []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS api_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			token_prefix TEXT NOT NULL,
			scopes TEXT NOT NULL,
			created_at TEXT NOT NULL,
			last_used_at TEXT,
			revoked_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS companies (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			name TEXT NOT NULL,
			website TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT '',
			version BIGINT NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS companies_user_name_lower ON companies(user_id, lower(name))`,
		`CREATE TABLE IF NOT EXISTS roles (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			company_id TEXT NOT NULL REFERENCES companies(id),
			title TEXT NOT NULL,
			location TEXT NOT NULL DEFAULT '',
			salary_text TEXT NOT NULL DEFAULT '',
			salary_min_cents BIGINT,
			salary_max_cents BIGINT,
			fit TEXT NOT NULL DEFAULT '',
			posting_url TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			job_type TEXT NOT NULL DEFAULT 'Full-time',
			version BIGINT NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS roles_company_idx ON roles(company_id)`,
		`CREATE TABLE IF NOT EXISTS applications (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			role_id TEXT NOT NULL REFERENCES roles(id),
			status TEXT NOT NULL,
			applied_date TEXT,
			next_step TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT '',
			resume_version TEXT NOT NULL DEFAULT '',
			resume_file_ref TEXT NOT NULL DEFAULT '',
			contact_email TEXT NOT NULL DEFAULT '',
			external_id TEXT NOT NULL DEFAULT '',
			version BIGINT NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS applications_user_external_uidx ON applications(user_id, external_id) WHERE external_id <> ''`,
		`CREATE INDEX IF NOT EXISTS applications_user_updated_idx ON applications(user_id, updated_at, id)`,
		`CREATE INDEX IF NOT EXISTS applications_user_status_idx ON applications(user_id, status)`,
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			from_status TEXT NOT NULL DEFAULT '',
			to_status TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			body TEXT NOT NULL DEFAULT '',
			occurred_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			actor_token_id TEXT NOT NULL DEFAULT '',
			actor_name TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS events_app_idx ON events(application_id, occurred_at, id)`,
		`CREATE TABLE IF NOT EXISTS tags (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			name TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS tags_user_name_lower ON tags(user_id, lower(name))`,
		`CREATE TABLE IF NOT EXISTS application_tags (
			application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (application_id, tag_id)
		)`,
		`CREATE TABLE IF NOT EXISTS idempotency_keys (
			token_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			idem_key TEXT NOT NULL,
			method TEXT NOT NULL,
			path TEXT NOT NULL,
			request_hash TEXT NOT NULL,
			status_code BIGINT NOT NULL,
			response_body TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY (token_id, idem_key)
		)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token_id TEXT NOT NULL DEFAULT '',
			actor_name TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL DEFAULT '',
			summary TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS audit_user_created_idx ON audit_log(user_id, created_at, id)`,
	},
}, {
	version: "002",
	stmts: []string{
		`CREATE TABLE IF NOT EXISTS work_items (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			state TEXT NOT NULL,
			lease_owner TEXT NOT NULL DEFAULT '',
			lease_until TEXT,
			attempts BIGINT NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS work_items_claim_idx ON work_items(user_id, state, created_at, id)`,
		`CREATE TABLE IF NOT EXISTS approvals (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			token_id TEXT NOT NULL,
			actor_name TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS approvals_app_idx ON approvals(application_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS artifacts (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			filename TEXT NOT NULL DEFAULT '',
			content_type TEXT NOT NULL DEFAULT '',
			body TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS artifacts_app_idx ON artifacts(application_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS questions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			prompt TEXT NOT NULL,
			answer TEXT NOT NULL DEFAULT '',
			required BIGINT NOT NULL DEFAULT 1,
			unknown BIGINT NOT NULL DEFAULT 0,
			source TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS questions_app_idx ON questions(application_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS standing_answers (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			answer_key TEXT NOT NULL,
			answer TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE (user_id, answer_key)
		)`,
		`CREATE TABLE IF NOT EXISTS company_caps (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			company TEXT NOT NULL,
			max_count BIGINT NOT NULL,
			window_days BIGINT NOT NULL,
			mode TEXT NOT NULL,
			UNIQUE (user_id, company)
		)`,
		`CREATE TABLE IF NOT EXISTS posting_checks (
			role_id TEXT PRIMARY KEY REFERENCES roles(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL,
			status TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			http_status BIGINT NOT NULL DEFAULT 0,
			checked_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS posting_checks_status_idx ON posting_checks(user_id, status)`,
		`CREATE TABLE IF NOT EXISTS webhooks (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			url TEXT NOT NULL,
			secret TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS leader_leases (
			name TEXT PRIMARY KEY,
			owner TEXT NOT NULL,
			lease_until TEXT NOT NULL
		)`,
	},
}, {
	version: "003",
	stmts: []string{
		`ALTER TABLE posting_checks ADD COLUMN changed BIGINT NOT NULL DEFAULT 0`,
	},
}, {
	version: "004",
	stmts: []string{
		`ALTER TABLE applications ADD COLUMN archived_at TEXT NOT NULL DEFAULT ''`,
	},
}, {
	version: "005",
	stmts: []string{
		`CREATE TABLE IF NOT EXISTS custom_fields (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS custom_fields_user_name_lower ON custom_fields(user_id, lower(name))`,
		`CREATE TABLE IF NOT EXISTS custom_field_values (
			application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			field_id TEXT NOT NULL REFERENCES custom_fields(id) ON DELETE CASCADE,
			value TEXT NOT NULL DEFAULT '',
			value_number REAL,
			PRIMARY KEY (application_id, field_id)
		)`,
	},
}, {
	version: "006",
	stmts: []string{
		`ALTER TABLE applications ADD COLUMN outcome TEXT NOT NULL DEFAULT ''`,
		// Outcomes used to overwrite the stage. Move them to outcome and
		// restore the stage from the last status change that led into them,
		// falling back to applied (or to_apply without an applied date).
		`UPDATE applications SET outcome=status, status=COALESCE(
			(SELECT e.from_status FROM events e WHERE e.application_id=applications.id AND e.to_status=applications.status
				AND e.from_status NOT IN ('', 'rejected', 'withdrawn', 'ghosted', 'failed', 'needs_input', 'waiting')
				ORDER BY e.occurred_at DESC, e.id DESC LIMIT 1),
			CASE WHEN applied_date IS NOT NULL AND applied_date <> '' THEN 'applied' ELSE 'to_apply' END)
			WHERE status IN ('rejected', 'withdrawn', 'ghosted', 'failed', 'needs_input')`,
		`CREATE TABLE IF NOT EXISTS interviews (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			stage TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT '',
			interviewer TEXT NOT NULL DEFAULT '',
			scheduled_at TEXT NOT NULL DEFAULT '',
			done_at TEXT NOT NULL DEFAULT '',
			prep TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT '',
			version BIGINT NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS interviews_app_idx ON interviews(application_id, created_at, id)`,
	},
}, {
	version: "007",
	stmts: []string{
		`ALTER TABLE companies ADD COLUMN icon_url TEXT NOT NULL DEFAULT ''`,
	},
}, {
	version: "008",
	stmts: []string{
		`ALTER TABLE users ADD COLUMN avatar TEXT NOT NULL DEFAULT ''`,
		// Documents belong to the user, not an application: the base résumé
		// prepare attaches, a cover-letter template, a LinkedIn PDF export.
		`CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			kind TEXT NOT NULL,
			filename TEXT NOT NULL DEFAULT '',
			content_type TEXT NOT NULL DEFAULT '',
			body TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS documents_user_idx ON documents(user_id, kind, created_at)`,
	},
}, {
	version: "009",
	stmts: []string{
		// OAuth2 grants for pursuit auth login. kind is code (loopback +
		// PKCE) or device. Codes are stored as SHA-256 hashes only.
		`CREATE TABLE IF NOT EXISTS oauth_grants (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			code_hash TEXT NOT NULL UNIQUE,
			user_code_hash TEXT UNIQUE,
			client_id TEXT NOT NULL,
			scopes TEXT NOT NULL,
			redirect_uri TEXT NOT NULL DEFAULT '',
			code_challenge TEXT NOT NULL DEFAULT '',
			user_id TEXT REFERENCES users(id),
			status TEXT NOT NULL,
			interval_seconds BIGINT NOT NULL DEFAULT 0,
			last_polled_at TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS oauth_grants_expires_idx ON oauth_grants(expires_at)`,
	},
}, {
	version: "010",
	stmts: []string{
		// The approving session's token, so the issued CLI token's audit
		// entry can name who let it in.
		`ALTER TABLE oauth_grants ADD COLUMN approver_token_id TEXT NOT NULL DEFAULT ''`,
	},
}, {
	version: "011",
	stmts: []string{
		// Tasks: the small things waiting on you, on any stage of an application.
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			stage TEXT NOT NULL,
			title TEXT NOT NULL,
			notes TEXT NOT NULL DEFAULT '',
			due_at TEXT NOT NULL DEFAULT '',
			done_at TEXT NOT NULL DEFAULT '',
			version BIGINT NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS tasks_app_idx ON tasks(application_id, created_at, id)`,
	},
}, {
	version: "012",
	stmts: []string{
		// The address a device login was started from, shown on /device.
		`ALTER TABLE oauth_grants ADD COLUMN requested_from TEXT NOT NULL DEFAULT ''`,
	},
}, {
	version: "013",
	stmts: []string{
		// Passkeys. The WebAuthn user handle is random, not the email or
		// users.id, so an authenticator holds nothing that names the person.
		`ALTER TABLE users ADD COLUMN webauthn_user_handle TEXT NOT NULL DEFAULT ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS users_webauthn_handle_uidx ON users(webauthn_user_handle) WHERE webauthn_user_handle <> ''`,
		// id is the credential ID (base64url). credential is go-webauthn's
		// whole Credential record as JSON: public key, sign count, flags,
		// AAGUID, transports, attestation.
		`CREATE TABLE IF NOT EXISTS webauthn_credentials (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			credential TEXT NOT NULL,
			created_at TEXT NOT NULL,
			last_used_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS webauthn_credentials_user_idx ON webauthn_credentials(user_id, created_at)`,
		// One row per begin, keyed by the challenge. Single use (used_at)
		// and short-lived (expires_at), like oauth_grants.
		`CREATE TABLE IF NOT EXISTS webauthn_challenges (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT '',
			session TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			used_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS webauthn_challenges_expires_idx ON webauthn_challenges(expires_at)`,
	},
}}
