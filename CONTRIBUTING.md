# Contributing to codeDB Lens

Thanks for helping! Contributions are accepted in English or Portuguese; code, commits and rule IDs are in English.

## Ground rules

1. **Read-only.** A rule may only read catalog views, DMVs and statistics. Anything that writes, or that
   needs more than the [minimum permissions](README.md#minimum-permissions), will be rejected or
   redesigned.
2. **No silent OK.** If a rule can't run (permission, version, timeout), it must report *No data* with the reason.
3. **Both languages.** Every user-facing text has `en` and `pt-BR`. CI fails if one is missing.
4. **Every rule has a test fixture** that provokes the finding, and a doc page in both languages.

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

Rules live in `rules/<engine>/<RULE-ID>/` as `rule.yaml` + `query.sql`. Copy an existing rule, change the
ID (`MSSQL-<AREA>-NNN` / `PG-<AREA>-NNN`), add a fixture in `testdata/fixtures/<engine>/` and a page in
`docs/en/rules/` and `docs/pt-BR/rules/`. Use `--rules-dir ./rules` to try it without recompiling.

## Pull requests

- One rule or one change per PR.
- Run `go test ./...` and `mkdocs build --strict` before pushing.
- Add a line to `CHANGELOG.md` under *Unreleased*.
