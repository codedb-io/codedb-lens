# Releasing codeDB Lens

## One-time setup

1. Repository secret `LENS_SIGNING_KEY`: the private Ed25519 seed from `go run ./tools/sign -genkey`. The matching public
   key is `PublicKey` in `internal/update/update.go`. Never commit the private key.
2. GitHub Pages: Settings → Pages → Source "GitHub Actions", custom domain `lens.codedb.com.br` (DNS: `CNAME lens` →
   `codedb-io.github.io`), then "Enforce HTTPS". The `Docs` workflow publishes on every change to `docs/`.
3. After the first image push: make the `codedb-lens` package on ghcr.io public (Package settings → Change visibility).
4. winget: the first version (`codeDB.Lens` 0.6.0) was submitted by hand to `microsoft/winget-pkgs`. For later
   versions, `publish.yml` runs `winget-releaser` when the repository secret `WINGET_TOKEN` exists: a classic PAT
   (`public_repo`) of an account that has a fork of `microsoft/winget-pkgs`. Without the secret, the job does nothing.
5. Scoop needs no setup: `publish.yml` rewrites `bucket/codedb-lens.json` on `main` when a release is published.
6. Windows code signing: not available for now (SignPath Foundation refused on 2026-10-22 for lack of public visibility;
   reapply later. Azure Artifact Signing does not serve Brazil). Chocolatey waits for signing.

## Before tagging

- [ ] CI green on `main` (unit, full SQL Server and PostgreSQL matrix, docs).
- [ ] `CHANGELOG.md`: move *Unreleased* to the new version with today's date.
- [ ] Lifecycle and build tables still correct (`MSSQL-VERSION-001`/`002`, `PG-VERSION-001`/`002` queries).
- [ ] Local dry run: `goreleaser release --snapshot --clean --skip=publish,sign`.
- [ ] Managed-services checklist below, at least for services touched by this release.
- [ ] Read-only audit below, when rules, collectors (`rules/<engine>/_collect`) or the adapters' session setup changed.
- [ ] Recorder overhead below, when collectors changed: run the `Overhead` workflow (30 min) and update the table here
      and in `docs/*/record.md`.
- [ ] Free key: `license.ConsentVersion` matches the consent text in `internal/license/consent.go` and the version
      agreed with the codeDB backend (a new text needs a new version); `license.Keys` has only production kids.
- [ ] Free key, against production with a test e-mail: `register` returns a key that `license status` accepts,
      registering again keeps working, and `compare --profile` produces the HTML. Delete the test record afterwards
      ("Delete my data" in the e-mail).
- [ ] `server.json`: `version` and the image tag in `identifier` set to the new version.
- [ ] MCP, when `internal/mcpserver` changed (plan 11, §7.9), on the release candidate:
  - [ ] In Claude Desktop, Claude Code, VS Code and Cursor: `mcp install`, restart, ask "run the diagnosis on profile X"
        and "which are the 5 biggest tables?". The report shows up in the profile's history.
  - [ ] With a data profile on a test database: "top 10 customers by revenue" and a comparison between periods; the
        numbers match the same SQL run by hand, and the AI reacts to `truncated`.
  - [ ] Follow the app's **AI (MCP)** guide and `docs/*/mcp.md` from scratch on a Windows with nothing configured.
  - [ ] `npx @modelcontextprotocol/inspector codedb-lens mcp`: list and call every tool.
  - [ ] Ask the AI to "drop table X" in the default mode: refused and explained, nothing changed.
  - [ ] Write mode on a throwaway database: double opt-in, a `CREATE INDEX` gets a preview, a confirmation, runs, and
        appears in `mcp-audit.jsonl`.
  - [ ] Linux: the image with `docker run -i` in Claude Code.
  - [ ] Read-only audit below during an MCP session in the default mode.

## Rotating the license key

The codeDB backend signs free keys. A new signing key gets a new `kid`: add its public key to `license.Keys`, release,
and only then switch the backend to it. Keep the old kid until the last key it signed has expired (90 days).

## Managed services (manual, no CI)

Run `codedb-lens check` with the minimum-permission login from `docs/en/permissions.md` and record the result in the release notes.

| Service | Check | Expected |
|---|---|---|
| Azure SQL Managed Instance | Platform detected as `azure_mi` | `MSSQL-VERSION-001` not applicable; others run |
| Amazon RDS for SQL Server | Platform detected as `rds` | Agent and backup rules run (RDS keeps msdb history) |
| Amazon RDS for PostgreSQL | Platform `rds` | All rules run; `PG-BLOAT-001` may be *No data* without `pg_read_all_data` |
| Aurora PostgreSQL | Platform `aurora` | Slots rule runs; size works per database |
| Azure Database for PostgreSQL Flexible | Platform `azure_flex` | All rules run |
| Azure SQL Database | Platform `azure_sqldb` | 6 of 13 rules run; no grade when few rules apply |

For each: coverage %, any *No data* and its reason, and anything surprising (open an issue).

Last recorded run per service:

| Service | Version | Result |
|---|---|---|
| Azure SQL Database | 0.5.0 | OK (6 of 12 rules) |
| Azure SQL Managed Instance | — | not run yet |
| Amazon RDS for SQL Server | — | not run yet |
| Amazon RDS for PostgreSQL | — | not run yet |
| Aurora PostgreSQL | — | not run yet |
| Azure Database for PostgreSQL Flexible | — | not run yet |

## Read-only audit (manual)

Capture every statement Lens sends and confirm there is no DDL/DML.

- PostgreSQL: start a server with `-c log_statement=all`, run `check`, and list the `execute`/`statement` lines of
  the server log. Expected: only `SELECT`/`WITH`.
- SQL Server: an Extended Events session with `sql_batch_completed` + `rpc_completed`, filtered by
  `sqlserver.client_app_name = N'codeDB Lens'`, ring buffer target; run `check` and read the target. Expected:
  `SELECT`/`WITH`, session `SET`, `USE <db>` and the driver's `exec sp_reset_connection`.

Last run: 0.6.0 candidate, 2026-10-22, PostgreSQL 17 (18 statements) and SQL Server 2022 (18 statements): no DDL/DML.
(sqlcmd reading the ring buffer needs `-I`: the XML methods require QUOTED_IDENTIFIER ON.)

For `record`, do the same with a recording of a few minutes: the collectors must show only `SELECT`/`WITH` too.
Last run for `record`: 0.8.0 candidate, 2026-10-23, 35 s recordings: PostgreSQL 17 (15 statements, all `SELECT`) and
SQL Server 2022 (19 `SELECT`s plus the session `SET`s): no DDL/DML.

## Recorder overhead

Target (Q5): the recorder uses less than 1% of the server's CPU. `tools/overhead` runs a synthetic OLTP load without
recording, with `codedb-lens record`, and without again, and reports the server time of the recorder's queries (they all
carry `/* codeDB Lens */`), the load's throughput and the recording size per hour.

```bash
CODEDB_LENS_PASSWORD=... go run ./tools/overhead -engine pg -server localhost -port 15417 -lens ./codedb-lens -minutes 30
```

Last run:

| | SQL Server 2022 | PostgreSQL 17 |
|---|---|---|
| Recorder share of the server's CPU | **0.003%** | **0.0001%** (execution time, an upper bound) |
| Recorder server time per hour | about 1.9 s | about 0.06 s |
| Load throughput without / with recording | 1953 / 1896 ops/s | 6879 / 6548 ops/s |
| Recording size on these test servers | about 0.04 MB per hour | about 0.04 MB per hour |

Measured on 2026-10-23 on one workstation (20 cores, Docker Desktop), 10 minutes recording between two 5-minute
periods without, both engines at the same time. On that machine the load generator, the databases and Lens share the
CPU, and throughput varied from +7% to −5% between runs with or without recording, so the server time of the
recorder's own queries is the direct measure. A busy server writes more per hour (more sessions and queries per sample).

## Tag and publish

```bash
git tag v0.1.0
git push origin v0.1.0
```

The `Release` workflow builds Windows and Linux archives, signs `checksums.txt`, builds the per-user installer and
attaches it to a **draft** release. Review the draft, check the assets and the signature file, then publish.

## After publishing

- [ ] Install from the installer on a clean Windows machine: Start menu icon, GUI opens, `codedb-lens version` in a terminal.
- [ ] From the previous version, `codedb-lens update` installs the new one (repository must be public, phase 2).
- [ ] `Publish` workflow green: `bucket/codedb-lens.json` has the new version; winget PR opened (if `WINGET_TOKEN`).
- [ ] Upload the installer and the Windows zip (downloaded from the release) to https://www.virustotal.com and link
      the results in the release notes. The unsigned Go build can trip machine-learning heuristics: report each
      detection to its vendor (https://docs.virustotal.com/docs/false-positive-contacts). Only if Microsoft Defender
      detects it, use https://www.microsoft.com/wdsi/filesubmission ("Incorrectly detected", with the detection name).
- [ ] MCP Registry, once the image is public: `mcp-publisher login github` (as a member of `codedb-io`), then
      `mcp-publisher publish` from the repository root. The image's `io.modelcontextprotocol.server.name` label must
      match `server.json`.
