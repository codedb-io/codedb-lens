# Releasing codeDB Lens

## One-time setup

1. Repository secret `LENS_SIGNING_KEY`: the private Ed25519 seed from `go run ./tools/sign -genkey`. The matching public
   key is `PublicKey` in `internal/update/update.go`. Never commit the private key.
2. Before the public launch (phase 2): Windows code signing (Azure Trusted Signing first option), then uncomment
   `nfpms`, `dockers_v2`, `winget` and `chocolateys` in `.goreleaser.yaml`.

## Before tagging

- [ ] CI green on `main` (unit, full SQL Server and PostgreSQL matrix, docs).
- [ ] `CHANGELOG.md`: move *Unreleased* to the new version with today's date.
- [ ] Lifecycle tables still correct (`MSSQL-VERSION-001`, `PG-VERSION-001` queries).
- [ ] Local dry run: `goreleaser release --snapshot --clean --skip=publish,sign`.
- [ ] Managed-services checklist below, at least for services touched by this release.
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

For each: coverage %, any *No data* and its reason, and anything surprising (open an issue).

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
