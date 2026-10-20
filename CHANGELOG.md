# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [SemVer](https://semver.org/).

## [Unreleased]

### Added
- Repository scaffolding, bilingual README, security policy, icon and development environment.
- `check` engine: data-only rule catalog (YAML + SQL), A–F grade with coverage seal, Confirmed / Hint / No data.
- SQL Server and PostgreSQL adapters with read-only, low-impact sessions.
- Rules: MSSQL-BACKUP-001, MSSQL-VERSION-001, PG-VERSION-001, PG-WRAPAROUND-001.
- Rules: MSSQL-CHECKDB-001, MSSQL-AGENT-001, MSSQL-SPACE-001, MSSQL-CONFIG-001, MSSQL-TEMPDB-001.

- Self-contained HTML report (English / Portuguese toggle) and JSON report.
- Saved connection profiles; passwords in the OS credential store.
- Windows app (same executable also works as CLI).
- Daily version check and `codedb-lens update` with Ed25519-signed checksums.

### Fixed
- Negative rule parameters no longer turn into SQL comments (`--1`).
