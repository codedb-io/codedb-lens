# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue. Use GitHub's
[private vulnerability reporting](https://github.com/codedb-io/codedb-lens/security/advisories/new).
We aim to acknowledge reports within 3 business days.

## What codeDB Lens does and does not do

- **Read-only.** It only runs `SELECT` statements and catalog/DMV queries. No DDL, no DML, no `EXEC` of
  user procedures. Sessions are opened with lock and statement timeouts, low deadlock priority (SQL Server)
  and `default_transaction_read_only = on` (PostgreSQL). Connections identify themselves as `codeDB Lens`.
- **No telemetry.** No data about your database, servers or usage leaves your machine.
- **One optional network call:** at most once a day, an anonymous `GET` to the GitHub Releases API to check
  for a newer version. Disable it with `--no-update-check` or `CODEDB_LENS_NO_UPDATE=1`.
- **Signed updates.** `codedb-lens update` only installs a binary whose SHA-256 matches `checksums.txt`
  and whose `checksums.txt` carries a valid Ed25519 signature from the key embedded in the binary.
- **Passwords** are never accepted as command-line arguments and are never written to disk in plain text.
  Saved profiles store them in the OS credential store (Windows Credential Manager, Linux Secret Service).
- **Reports** may contain server names, database names and query text. Treat them as internal documents.

## Supported versions

Only the latest release receives security fixes.
