# Releasing codeDB Lens

## One-time setup

1. Repository secret `LENS_SIGNING_KEY`: the private Ed25519 seed from `go run ./tools/sign -genkey`. The matching public
   key is `PublicKey` in `internal/update/update.go`. Never commit the private key.
2. GitHub Pages: Settings → Pages → Source "GitHub Actions", custom domain `lens.codedb.com.br` (DNS: `CNAME lens` →
   `codedb-io.github.io`), then "Enforce HTTPS". The `Docs` workflow publishes on every change to `docs/`.
3. After the first image push: make the `codedb-lens` package on ghcr.io public (Package settings → Change visibility).
4. After Windows code signing (SignPath Foundation; Azure Trusted Signing as plan B): uncomment `winget` and
   `chocolateys` in `.goreleaser.yaml`.

## Before tagging

- [ ] CI green on `main` (unit, full SQL Server and PostgreSQL matrix, docs).
- [ ] `CHANGELOG.md`: move *Unreleased* to the new version with today's date.
- [ ] Lifecycle tables still correct (`MSSQL-VERSION-001`, `PG-VERSION-001` queries).
- [ ] Local dry run: `goreleaser release --snapshot --clean --skip=publish,sign`.
- [ ] Managed-services checklist below, at least for services touched by this release.
- [ ] Read-only audit below, when rules or the adapters' session setup changed.
- [ ] Free key: `license.ConsentVersion` matches the consent text in `internal/license/consent.go` and the version
      agreed with the codeDB backend (a new text needs a new version); `license.Keys` has only production kids.
- [ ] Free key, against production with a test e-mail: `register` returns a key that `license status` accepts,
      registering again keeps working, and `compare --profile` produces the HTML. Delete the test record afterwards
      ("Delete my data" in the e-mail).

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
| Azure SQL Database | Platform `azure_sqldb` | 6 of 12 rules run; no grade when few rules apply |

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

Last run: 0.5.1 candidate, 2026-10-22, PostgreSQL 17 (18 statements) and SQL Server 2022 (20 statements): no DDL/DML.

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
