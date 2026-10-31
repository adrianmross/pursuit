# Contributing

Commit messages on `main` use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add URL lookup for the open posting
fix(cli): return exit code 3 when an application is missing
docs: describe extension pairing
chore: ignore extension build output
```

Use `feat`, `fix`, `docs`, or `chore`. A scope is optional (`feat(cli):`, `fix(extension):`). The subject is imperative. Do not add attribution trailers.

## Releases

pursuit is pre-release while it is `0.x`. One version covers the CLI, server, API, dashboard, docs and extension.

A PR that changes user-visible behaviour carries a fragment from [changelogger](https://github.com/adrianmross/changelogger):

```
go install github.com/adrianmross/changelogger/cmd/changelogger@v0.10.0
changelogger
```

It asks for the bump and a one-line note, writes `.changelogs/<slug>.md`, and prints the PR title to use (`feat(pursuit): ...`). CI fails a PR whose title does not match its fragment. Pre-1.0, `fix` bumps the patch and `feat` or a breaking change bumps the minor. PRs touching only `.github/`, `docs/` or `*.md` need no fragment.

Schema changes are new migrations appended to `migrations` in `internal/sqlstore/migrate.go`, never edits to a merged one: a database skips a version it has already recorded. `TestMigrationsAreAppendOnly` checks each migration against `internal/sqlstore/testdata/migrations.txt`; record a new one with `UPDATE_MIGRATIONS=1 go test ./internal/sqlstore/ -run TestMigrationsAreAppendOnly`. A PR that adds a migration needs `bump: minor` (or major), and CI enforces it.

After merge, Release Please opens a `chore(release): pursuit vX.Y.Z` PR that bumps every version string and `CHANGELOG.md`. Merging it tags the release and publishes binaries and the extension zip to [GitHub Releases](https://github.com/adrianmross/pursuit/releases); the [Homebrew tap](https://github.com/adrianmross/homebrew-tap) picks the release up.
