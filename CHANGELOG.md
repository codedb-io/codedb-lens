# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [SemVer](https://semver.org/).

## [Unreleased]

### Added
- `mcp`: MCP server over stdio for AI clients (Claude, VS Code, Cursor), read-only, in the same binary. First tools:
  `list_profiles` (never returns passwords or connection strings), `list_rules` and `explain_rule`. `--profile a,b`
  limits which saved profiles the AI sees. Uses the official Go SDK (`github.com/modelcontextprotocol/go-sdk`).

## [0.8.0] - 2026-10-23

### Added
- `record`: samples the instance every 15 s (sessions, blocking, waits, counters, worker threads or connections) and
  every 5 min (top queries by delta) into hourly `.jsonl.gz` segments, for 1 hour, or 24 hours with the free key (one
  instance per key, like `compare`). Same read-only permissions as `check`; nothing is created on the server. Reconnects
  by itself (gaps become *No data*), stops cleanly on Ctrl+C or `SIGTERM`, at the deadline or at 500 MB. One recording
  at a time per computer; Windows does not sleep while it records.
- `report <folder> [--from --to] [--redact]`: self-contained HTML + JSON of a recording, also while it runs: time with
  data (the recording's coverage seal), findings as episodes, charts over time with no-data and episode bands, and the
  top queries of the period. Every technical number has an **i** button explaining it (native popover, en/pt-BR).
- 8 series rules (`*-REC-BLOCKING-001`, `*-REC-WAITS-001`, `*-REC-LONGQUERY-001`, `MSSQL-REC-WORKERS-001`,
  `PG-REC-CONN-001`): YAML that says which collector to read, what to measure and the limit. No A–F grade for recordings.
- GUI: the **Record** tab (1 or 24 hours, live progress, stop, report, list of recordings, command to copy for a server).
- `tools/overhead` and the manual `Overhead` workflow measure the recorder's cost under load.

### Changed
- The check catalog version ignores `rules/<engine>/_collect`, so editing a collector does not change it.

## [0.7.0] - 2026-10-23

### Added
- GUI: **Update now** button on the new-version banner. Same Ed25519-verified path as `codedb-lens update`, then the
  app restarts. Package-manager installs keep the command hint. The check still runs at most once a day.
- GUI: **Home** screen on open (how it works, read-only notice, shortcuts, codeDB and free key). No profile is
  selected and no connection is attempted at startup.
- GUI: a selected connection has **Diagnosis**, **Record** (coming in v0.8) and **History** tabs.
- GUI: delete a run from the history (its `.html` and `.json`, permanently, after confirmation).

### Changed
- GUI: the codeDB call to action rotates between six messages every 20 s (paused on hover); the link carries
  `utm_content` so the SaaS can tell which message was clicked.

## [0.6.1] - 2026-10-23

### Added
- Build provenance attestations for every release asset: `gh attestation verify <file> --repo codedb-io/codedb-lens`.
- Scoop bucket: `scoop bucket add codedb https://github.com/codedb-io/codedb-lens`, then `scoop install codedb/codedb-lens`.
- Docs page on the Windows SmartScreen warning and how to verify a download.

### Fixed
- With `--dsn-env`, the CLI said "Connecting to localhost..."; it now shows the host from the connection string.

## [0.6.0] - 2026-10-22

### Added
- Executive summary: the HTML report opens on a **Summary** tab (grade and coverage, highest open severity, top 3 risks,
  evolution against the previous run of the same instance) with a one-page A4 **Print summary**; the full report is on
  the **Technical** tab. Evolution needs the free key; the JSON gains `evolution`.
- `check --redact` (CLI): writes only `<timestamp>.redacted.html/.json`, masking host, databases, objects, logins,
  applications, client addresses, Agent jobs, slots and paths with deterministic tokens (HMAC with a per-machine salt in
  `redact.salt`) and replacing query texts and Agent step messages with `[redacted]`. Redacted runs compare only with
  redacted runs. JSON gets `redacted: true`.
- Every rule has a `verify` field ("How to verify the fix"), shown in the report and JSON and on each rule page.
- Rules MSSQL-VERSION-002 (build behind the latest CU/GDR) and PG-VERSION-002 (minor version behind the latest), with
  bundled build tables as of 2026-10-22. The catalog now has 26 rules (13 per engine).

### Changed
- No grade (**?**) now means fewer than 6 applicable rules, instead of fewer than half of the catalog: Azure SQL Database
  (6 of 13) keeps its grade as rules that never apply there are added.

### Fixed
- MSSQL-VERSION-001 now recognizes SQL Server 2025.
- Windows app: clicking a finding in the report's lists opened a broken page instead of scrolling to it, and **Print
  summary** did nothing.

## [0.5.1] - 2026-10-22

### Added
- Documentation site at https://lens.codedb.com.br/ (GitHub Pages), so the "Documentation" links in reports resolve.
- Linux `.deb` and `.rpm` packages, and a container image `ghcr.io/codedb-io/codedb-lens` (amd64, arm64).

### Fixed
- Free key validity in the singular: "1 day left" / "falta 1 dia".
- GUI profiles: names that differ only in case are the same profile (Windows credential store and report folders
  ignore case); a failed rename no longer drops the saved password.

## [0.5.0] - 2026-10-22

### Added
- Azure SQL Database: waits, top queries, missing and unused indexes and space against the database max size now run
  (6 of 12 rules; the rest are managed by Azure). Rules can ship `query.<platform>.sql` for platform-specific DMVs.
  Connecting to `master` stops with a clear message, since every DMV there is about `master`.
- No grade (**?**) when fewer than half of the rules apply to the platform, instead of a grade over one or two checks.
- `check --fail-on <severity>` and `--min-coverage <pct>` for CI and scheduled jobs. Exit codes: 0 ok, 1 findings,
  2 error, 3 coverage below the minimum.
- Reports record the rule catalog fingerprint (`catalog_version`) and the run duration; `compare` uses the fingerprint
  to warn when the catalog changed.
- Docs: how the grade is calculated, CI usage, and a sample report and comparison from synthetic data
  (`go run ./tools/demo`).
- Catalog test rejects rule queries with write or session-changing keywords.
- `compare`: new, resolved and still-open findings between two runs of the same instance, with grade and coverage side by
  side and warnings when coverage, Lens version or the rule catalog changed. HTML (en/pt-BR) and JSON. App: **Compare
  with previous** in the history.
- Free key: `codedb-lens register` and **Get the free key** in the app (name and e-mail, sent once, only on request);
  Ed25519 license verified offline; `codedb-lens license [status|import|renew|reset|remove]`; one instance per key.
  Renewing needs no form (**Renew key** in the app), from 14 days before expiry to 30 days after.
- Repository scaffolding, bilingual README, security policy, icon and development environment.
- `check` engine: data-only rule catalog (YAML + SQL), A–F grade with coverage seal, Confirmed / Hint / No data.
- SQL Server and PostgreSQL adapters with read-only, low-impact sessions.
- Rules: MSSQL-BACKUP-001, MSSQL-VERSION-001, PG-VERSION-001, PG-WRAPAROUND-001.
- Rules: MSSQL-CHECKDB-001, MSSQL-AGENT-001, MSSQL-SPACE-001, MSSQL-CONFIG-001, MSSQL-TEMPDB-001.
- Rules: MSSQL-WAITS-001, MSSQL-BLOCKING-001, MSSQL-TOPQUERY-001, MSSQL-MISSINGIDX-001, MSSQL-UNUSEDIDX-001.
- Rules: PG-PGSS-001, PG-CONN-001, PG-SLOTS-001, PG-SIZE-001, PG-LONGTX-001.
- Rules: PG-LOCKS-001, PG-TOPQUERY-001, PG-AUTOVAC-001, PG-INDEXES-001, PG-BLOAT-001.
- Per-database rules (`scope: database`): databases the login can't open are reported, never hidden.
- Bilingual evidence columns (`name@en` / `name@pt-BR`).
- Minimum SQL Server permissions now include `CONNECT ANY DATABASE` (no data access).
- Self-contained HTML report (English / Portuguese toggle) and JSON report.
- Saved connection profiles; passwords in the OS credential store.
- Windows app (same executable also works as CLI).
- Daily version check and `codedb-lens update` with Ed25519-signed checksums.
- Docs: getting started and roadmap pages; RELEASING.md with the managed-services checklist.

### Changed
- Free key consent text approved as version `lens-2026-1` (same wording as the draft).
- Release files have no version in their names (`codedb-lens_windows_amd64.zip`, `codedb-lens_windows_amd64_setup.exe`,
  `codedb-lens_linux_<arch>.tar.gz`), so `releases/latest/download/<file>` links stay valid. `codedb-lens update`
  looks for the new names.
- Errors now exit with code 2 (was 1), so 1 can mean "findings" in pipelines.

### Fixed
- Azure SQL Database reported grade A with 100% coverage after running a single rule.
- "Findings N" and "Finding i of N" in the report counted only critical and high findings.
- "Server up since" was the server's clock time, hours off when the server runs in another time zone (e.g. UTC
  containers). It is now stored in UTC and shown in local time.
- The app opens reports in its own language, and new runs from the app default to it.
- Singular counts: "1 finding", "1 critical or high" (and pt-BR "1 achado", "1 crítico ou alto").
- Negative rule parameters no longer turn into SQL comments (`--1`).
- Two runs in the same minute no longer overwrite each other's report.
- Rule texts containing ": " are quoted so the YAML loads.
