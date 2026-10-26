# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [SemVer](https://semver.org/).

## [Unreleased]

## [0.9.0] - 2026-10-24

### Added
- `mcp`: MCP server over stdio for AI clients (Claude, VS Code, Cursor), read-only, in the same binary. First tools:
  `list_profiles` (never returns passwords or connection strings), `list_rules` and `explain_rule`. `--profile a,b`
  limits which saved profiles the AI sees. Uses the official Go SDK (`github.com/modelcontextprotocol/go-sdk`).
- `mcp` diagnosis tools: `check` (saves the report in the profile's history, like the app), `get_finding`, `history`,
  `compare` (free key), `list_recordings`, `recording_report` (a period of a recording) and `record_command` (the AI
  never starts a recording; it gives the user the command). Prompts `diagnose` and `incident`. `--redact` masks names in
  the results. Without saved profiles (servers, Docker), `mcp --engine ... --server ...` takes the connection from flags
  and `CODEDB_LENS_PASSWORD`. A whole MCP session is tested to make no HTTP call.
- `mcp` data tools: `list_databases`, `list_tables` (with whether the login can read each table), `describe_table`
  (columns, primary and foreign keys, indexes) and `query`, one read-only `SELECT` with four layers: a text guard (one
  statement, no write keyword or function with side effects; a 150-case attack corpus), a transaction that is always
  rolled back (PostgreSQL also read-only), limits (500 rows, 2 KB per cell, 10 s by default; `mcp_max_rows` and
  `mcp_timeout_s` per profile up to 10,000 rows and 120 s; a cut result says `truncated: true`), and a warning when the
  login itself can write. Prompt `explore_data`.
- `mcp install|uninstall --client claude-desktop|vscode|cursor [--profile a,b] [--redact] [--dry-run]`: adds or removes
  only the `codedb-lens` entry in the client's settings, with the absolute path of the Lens, keeps everything else,
  saves a `.bak`, leaves invalid JSON untouched (and prints the snippet to paste) and changes nothing on a second run.
  For Claude Code it prints the `claude mcp add` command instead. `mcp status` shows which clients have the Lens and
  whether the recorded path still exists.
- GUI: "AI (MCP)" screen with a step-by-step "How to use" guide and a "Configuration" tab (which clients have the Lens,
  the command and the JSON to copy for the profiles you tick); a "Use with AI" card on Home. The GUI installs nothing.
  Connections get "Max rows for the AI" and "Max query time" fields.
- `mcp` write mode, documented only in the docs (`mcp-write`): `execute_sql` exists only when the profile allows it
  (`codedb-lens mcp allow-write`, which must run in a terminal and asks you to type the profile name; `revoke-write`
  turns it off) and the client
  runs `mcp --allow-write`. The user confirms every call in the AI client, `dry_run` (the default) rolls back,
  statements without a transaction need `dry_run: false`, a deny list holds even then (logins, permissions, server
  settings, `DROP DATABASE`, OS and file access, transaction control, impersonation, dynamic SQL), and every attempt is written to `mcp-audit.jsonl`
  (no log, no write). The app never turns it on and shows a warning while any profile allows it.
- Docs: "Use with AI (MCP)" (set up and ask, use with data, MCP with Docker), in English and Portuguese; README
  section with a demo GIF. `server.json` and the image label for the official MCP Registry (`io.github.codedb-io/codedb-lens`).

### Fixed
- Windows: typed in PowerShell or cmd, commands that ask something (`mcp allow-write`, `key register`) lost the
  keyboard to the shell, which does not wait for the app's GUI `.exe`. The zip and the installer now carry
  `codedb-lens.com`, a console twin that terminals pick first for `codedb-lens` and wait for; `update` installs it too.
- The app's AI (MCP) screen shows the install command in PowerShell syntax (`& "..."`), using `codedb-lens.com`.
- `mcp status`, `install`, `uninstall`, `allow-write` and `revoke-write` accept `--lang`.
- `mcp`: with `--allow-write` but no profile allowed, `list_profiles` says what is missing. The prompts ask the AI to
  answer in the user's language instead of following their English text.

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
