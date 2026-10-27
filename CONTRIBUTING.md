# Contributing to codeDB Lens

Thanks for helping! Contributions are accepted in English or Portuguese; code, commits and rule IDs are in English.

New here? Issues labelled [`good first issue`](https://github.com/codedb-io/codedb-lens/labels/good%20first%20issue)
are rule ideas scoped for a first PR. Questions and rule ideas that need discussion go to
[Discussions](https://github.com/codedb-io/codedb-lens/discussions); bugs and concrete proposals go to issues.

## Ground rules

1. **Read-only.** A rule may only read catalog views, DMVs and statistics. Anything that writes, or that
   needs more than the [minimum permissions](README.md#minimum-permissions), will be rejected or
   redesigned.
2. **No silent OK.** If a rule can't run (permission, version, timeout), it must report *No data* with the reason.
3. **Both languages.** Every user-facing text has `en` and `pt-BR`. CI fails if one is missing.
4. **Every rule has an integration test** that provokes the finding (and one where it passes), and a doc page in
   both languages.

## Development setup

Open the folder in VS Code and accept the recommended extensions. You need Go (latest stable), Docker
and Python 3 (docs and icon only).

```bash
docker compose up -d                     # SQL Server 2022 + PostgreSQL 17
docker compose --profile matrix up -d    # full version matrix
go test ./...
go test -tags integration ./...
```

## Adding a rule

Rules live in `rules/<engine>/<RULE-ID>/` as `rule.yaml` + `query.sql`; no Go code is needed for a new rule.

1. Copy a close rule (for example `rules/pg/PG-AUTOVAC-001/`) and change the ID (`MSSQL-<AREA>-NNN` /
   `PG-<AREA>-NNN`).
2. `rule.yaml`: `severity`, `confidence` (`confirmed` or `hint`), `scope: database` only when the query must run in
   each database, the minimum `permission`, `params` (thresholds, referenced in the query as `{{name}}`, numbers
   only), and the texts in `en` and `pt-BR`: `title`, `impact`, `next_action`, `verify` (how to check the fix) and
   `caveat`.
3. `query.sql`: one read-only `SELECT`/`WITH`. Every row returned is evidence of a finding; no rows means OK. An
   optional `_confidence` column (`confirmed`/`hint`) sets the confidence (the strongest row wins). Cap the rows
   (`TOP`/`LIMIT 20`).
4. Try it without recompiling: `codedb-lens check --profile <yours> --rules-dir ./rules`.
5. Add a subtest in `internal/adapter/integration_test.go` that sets up the problem on the test containers and
   expects the finding, and a page in `docs/en/rules/` and `docs/pt-BR/rules/` (copy a neighbour).
6. `go test ./...` validates the catalog: both languages, params used by the query, unique IDs, no write keywords.

When a platform needs a different query (Azure SQL Database has database-scoped DMVs, for example), add
`query.<platform>.sql` next to `query.sql` with the same columns and params; Lens runs it on that platform and the
report shows the query that actually ran. Platforms: `onprem`, `azure_sqldb`, `azure_mi`, `rds` (SQL Server);
`onprem`, `rds`, `aurora`, `azure_flex` (PostgreSQL).

## Pull requests

- One rule or one change per PR.
- Run `go test ./...` and `mkdocs build --strict` before pushing.
- Add a line to `CHANGELOG.md` under *Unreleased*.
