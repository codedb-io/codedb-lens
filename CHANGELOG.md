# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [SemVer](https://semver.org/).

## [Unreleased]

### Added
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
- "Server up since" was the server's clock time, hours off when the server runs in another time zone (e.g. UTC
  containers). It is now stored in UTC and shown in local time.
- The app opens reports in its own language, and new runs from the app default to it.
- Singular counts: "1 finding", "1 critical or high" (and pt-BR "1 achado", "1 crítico ou alto").
- Negative rule parameters no longer turn into SQL comments (`--1`).
- Two runs in the same minute no longer overwrite each other's report.
- Rule texts containing ": " are quoted so the YAML loads.
