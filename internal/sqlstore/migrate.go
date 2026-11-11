package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
)

// advisoryLockKey is the Postgres session lock that serializes migrations and bootstrap.
const advisoryLockKey int64 = 594132718233451

// lockTx holds advisoryLockKey until tx ends. It is a transaction lock, not a
// session lock, so commit, rollback, or a dropped connection releases it, and
// it stays on one backend behind a transaction-mode pooler (Neon's pooled URL,
// PgBouncer). A session lock there can leak and hang every later boot.
// lock_timeout turns a stuck holder into an error instead of a 504.
func lockTx(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout = '15s'"); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", advisoryLockKey)
	return err
}

func Migrate(ctx context.Context, db *sql.DB, dialect string) error {
	if dialect == "postgres" {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := lockTx(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration lock: %w", err)
		}
		if err := applyMigrations(ctx, tx, dialect); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	if err := applyMigrations(ctx, conn, dialect); err != nil {
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

func applyMigrations(ctx context.Context, conn querier, dialect string) error {
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
		for _, stmt := range m.stmts {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migration %s: %w", m.version, err)
			}
		}
		ins := rebind(dialect, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`)
		if _, err := conn.ExecContext(ctx, ins, m.version, "1970-01-01T00:00:00.000000000Z"); err != nil {
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
}, {
	version: "014",
	stmts: []string{
		// The application this one is tracked with: one process at a company
		// covering several postings. Empty when it stands on its own.
		`ALTER TABLE applications ADD COLUMN tracked_by TEXT NOT NULL DEFAULT ''`,
	},
}, {
	version: "015",
	stmts: []string{
		// A round's prep and notes become one markdown document, prep first
		// under its own heading. ponytail: the prep column stays, unread,
		// since dropping a column differs per dialect.
		`UPDATE interviews SET notes = '## Prep
' || prep || CASE WHEN notes <> '' THEN '

## Notes
' || notes ELSE '' END, prep = '' WHERE prep <> ''`,
	},
}, {
	version: "016",
	stmts: []string{
		// Whether a round's notes show expanded in the application's notes
		// outline, or fold to one line.
		`ALTER TABLE interviews ADD COLUMN in_notes BIGINT NOT NULL DEFAULT 1`,
	},
}, {
	version: "017",
	stmts: []string{
		// A task can belong to one round of its stage; empty means the stage.
		`ALTER TABLE tasks ADD COLUMN round_id TEXT NOT NULL DEFAULT ''`,
	},
}, {
	version: "018",
	stmts: []string{
		// This application's own names for its stages, and the stages its
		// process skips, as JSON {labels, off}. Empty means the defaults.
		`ALTER TABLE applications ADD COLUMN stage_plan TEXT NOT NULL DEFAULT ''`,
	},
}, {
	version: "019",
	stmts: []string{
		// Notes files of their own on an application, beside its notes and
		// its rounds' notes: salary research, a referral thread, anything.
		`CREATE TABLE IF NOT EXISTS notes (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			body TEXT NOT NULL DEFAULT '',
			version BIGINT NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS notes_app_idx ON notes(application_id, created_at, id)`,
	},
}, {
	version: "020",
	stmts: []string{
		// One Stripe customer per user, and the subscription as last read
		// from Stripe. Rows are overwritten by a sync, never patched from a
		// webhook payload. status is "none" before the first subscription.
		`CREATE TABLE IF NOT EXISTS billing_accounts (
			user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			stripe_customer_id TEXT NOT NULL UNIQUE,
			subscription_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'none',
			price_id TEXT NOT NULL DEFAULT '',
			current_period_end TEXT NOT NULL DEFAULT '',
			cancel_at_period_end INTEGER NOT NULL DEFAULT 0,
			synced_at TEXT NOT NULL DEFAULT ''
		)`,
		// Features a user may use. source "stripe" rows mirror Stripe's
		// active entitlements and are replaced on every sync; "manual" rows
		// are comps a sync never touches.
		`CREATE TABLE IF NOT EXISTS entitlements (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			feature TEXT NOT NULL,
			source TEXT NOT NULL,
			granted_at TEXT NOT NULL,
			PRIMARY KEY (user_id, feature, source)
		)`,
		// The user's own on/off for an experimental feature.
		`CREATE TABLE IF NOT EXISTS user_features (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			feature TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (user_id, feature)
		)`,
		// Webhook deliveries already handled; Stripe redelivers and doesn't order.
		`CREATE TABLE IF NOT EXISTS stripe_events (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			received_at TEXT NOT NULL
		)`,
	},
}, {
	version: "021",
	stmts: []string{
		// Where the file is: "db" keeps it in body, "s3" in object_key of
		// the configured bucket. Old rows stay in the database.
		`ALTER TABLE documents ADD COLUMN storage TEXT NOT NULL DEFAULT 'db'`,
		`ALTER TABLE documents ADD COLUMN object_key TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN size INTEGER NOT NULL DEFAULT 0`,
		// Binary bodies are base64: about 3 bytes per 4 characters.
		`UPDATE documents SET size = CASE WHEN content_type LIKE 'text/%' THEN LENGTH(body) ELSE LENGTH(body) * 3 / 4 END`,
		// A name for a variant ("Backend", "Short"); the filename otherwise.
		`ALTER TABLE documents ADD COLUMN label TEXT NOT NULL DEFAULT ''`,
		// The main résumé and the main cover letter: one of each per user.
		`ALTER TABLE documents ADD COLUMN is_primary INTEGER NOT NULL DEFAULT 0`,
		// The default cover letter was a standing answer; it's a document now.
		`INSERT INTO documents (id, user_id, kind, filename, content_type, body, created_at, label, size)
			SELECT 'cl-' || id, user_id, 'cover_letter', 'cover-letter.md', 'text/markdown', answer, updated_at, 'Default', LENGTH(answer)
			FROM standing_answers WHERE answer_key = 'cover_letter' AND answer <> ''`,
		`DELETE FROM standing_answers WHERE answer_key = 'cover_letter'`,
		// Prepare used the newest of each; that one is the main one.
		`UPDATE documents SET is_primary = 1 WHERE kind IN ('base_resume', 'cover_letter') AND id = (
			SELECT d2.id FROM documents d2 WHERE d2.user_id = documents.user_id AND d2.kind = documents.kind
			ORDER BY d2.created_at DESC, d2.id DESC LIMIT 1)`,
	},
}, {
	version: "022",
	stmts: []string{
		// Smart search: one row per chunk of a source (an answer, a round's
		// notes, a text document), with its embedding. hash covers the
		// model and text, so a change to either re-embeds just that chunk.
		// vector is base64 of little-endian float32s.
		`CREATE TABLE IF NOT EXISTS search_chunks (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			source TEXT NOT NULL,
			source_id TEXT NOT NULL,
			idx INTEGER NOT NULL,
			title TEXT NOT NULL,
			text TEXT NOT NULL,
			hash TEXT NOT NULL,
			vector TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (user_id, source, source_id, idx)
		)`,
		// The closest saved answer to a form question prepare couldn't
		// match exactly. A suggestion only: the question stays unknown.
		`ALTER TABLE questions ADD COLUMN suggested_answer TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE questions ADD COLUMN suggested_source TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE questions ADD COLUMN suggested_score REAL NOT NULL DEFAULT 0`,
	},
}, {
	version: "023",
	stmts: []string{
		// Which of the user's documents an application's résumé or cover
		// letter was copied from; empty for a file uploaded to the application.
		`ALTER TABLE artifacts ADD COLUMN source_document_id TEXT NOT NULL DEFAULT ''`,
	},
}, {
	version: "024",
	stmts: []string{
		// The time of the newest feed announcement this user dismissed
		// (RFC 3339, UTC), so a dismissal follows the account across devices.
		`ALTER TABLE users ADD COLUMN announcements_seen TEXT NOT NULL DEFAULT ''`,
	},
}, {
	version: "025",
	stmts: []string{
		// Where a dashboard sign-in came from, so Settings can tell sessions apart.
		`ALTER TABLE api_tokens ADD COLUMN client TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE api_tokens ADD COLUMN ip TEXT NOT NULL DEFAULT ''`,
	},
}, {
	version: "026",
	stmts: []string{
		// OAuth clients that are not the CLI. source is cimd (a client
		// metadata document at an https client_id, refetched after
		// expires_at) or dcr (registered through /v1/auth/register).
		// redirect_uris is newline separated.
		`CREATE TABLE IF NOT EXISTS oauth_clients (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			redirect_uris TEXT NOT NULL,
			source TEXT NOT NULL,
			created_at TEXT NOT NULL,
			last_used_at TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL DEFAULT ''
		)`,
	},
}}
