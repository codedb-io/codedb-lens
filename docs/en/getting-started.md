# Getting started

## 1. Create a read-only login

Use the scripts in [Minimum permissions](permissions.md). Lens never writes to your database, and the login doesn't need
more than that.

## 2. Install

Download the files from [GitHub Releases](https://github.com/codedb-io/codedb-lens/releases/latest) or the
[codeDB Lens page](https://app.codedb.com.br/lens?utm_source=lens&utm_medium=docs).

- **Windows:** run `codedb-lens_windows_amd64_setup.exe`. It installs for your user only (no admin) and adds
  **codeDB Lens** to the Start menu. A portable `.zip` is also available. The installer is not code-signed yet, so
  Windows SmartScreen may warn: choose **More info → Run anyway**. See [Why Windows shows a warning](windows-warning.md)
  for the reason, how to verify the download and alternatives without the warning.
- **Windows (Scoop):** `scoop bucket add codedb https://github.com/codedb-io/codedb-lens`, then
  `scoop install codedb/codedb-lens`.
- **Linux (x64 / arm64):** install the `.deb` or `.rpm`, or extract the `.tar.gz` and put `codedb-lens` in your `PATH`.
- **Docker:** `docker run --rm -e PG_DSN -v "$PWD:/out" ghcr.io/codedb-io/codedb-lens check --engine pg --dsn-env PG_DSN --out /out`.

## 3. Run

### App (Windows)

1. Open **codeDB Lens** and pick **SQL Server** or **PostgreSQL**.
2. Fill in the server (or paste a connection string) and the login.
3. **Test connection**, then **Run diagnosis**. The report opens in the app; the connection is saved for next time and
   the password goes to the Windows Credential Manager.

### Command line (Windows and Linux)

```bash
# SQL Server, Windows authentication
codedb-lens check --engine mssql --server sql01 --auth windows

# SQL Server, SQL login (password from the environment)
CODEDB_LENS_PASSWORD='…' codedb-lens check --engine mssql --server sql01,1433 --user codedb_lens

# PostgreSQL, connection string in an environment variable
PG_DSN='postgres://codedb_lens:…@pg01:5432/postgres' codedb-lens check --engine pg --dsn-env PG_DSN

# Save the settings as a profile, then reuse it
codedb-lens check --engine mssql --server sql01 --auth windows --save prod
codedb-lens check --profile prod
```

Useful flags: `--lang en|pt-BR`, `--out <folder>`, `--trust-server-cert` (self-signed SQL Server certificates),
`--sslmode` (PostgreSQL), `--redact` (see [below](#5-share-a-report-safely)), `--no-update-check`.

## 4. Read the report

Reports are saved as HTML and JSON in `Documents\codeDB Lens\reports\<profile>` (Windows) or
`~/codedb-lens/reports/<profile>` (Linux). See [how to read the grade and coverage](index.md#how-to-read-the-report).
Each finding links to its rule page with the reasoning, the limits and what to do next.

The HTML report opens on a **Summary** tab: grade and coverage, the highest open severity, the top 3 risks (by severity,
then confirmed before hint) and the evolution against the previous run of the same instance in the same folder. The
evolution needs the [free key](compare.md), the same as `compare`; without it the summary shows a short invitation, and
the first run says "First diagnosis". **Print summary** prints only that tab on one A4 page (use the browser's
print-to-PDF to send it to a manager or client). The **Technical** tab has the full report, and each finding has a
**How to verify the fix** note. The JSON gains an `evolution` field.

## 5. Share a report safely

Reports contain server, database, object and login names. To send one to a consultant, a vendor or a client, add
`--redact` (command line only):

```bash
codedb-lens check --profile prod --redact
```

With `--redact`, **only** `<timestamp>.redacted.html` and `.redacted.json` are written; the unmasked report is not saved.

- **Masked** with stable tokens such as `db-3fa91c07`: host, databases (system databases stay visible), schemas,
  tables, indexes, logins, users, applications, client addresses, SQL Agent jobs, replication slots, and file paths.
  The same name always gets the same token within a report, so the evidence still reads coherently.
- **Removed:** query texts and SQL Agent step messages become `[redacted]`. Raw error messages get the same masking.
- **Not masked:** versions, numbers, severities, dates, the rule SQL, and system database names.
- **Salt:** tokens are an HMAC with a random per-machine salt stored in `redact.salt` next to `profiles.json`, so they
  can't be reversed by guessing names. The same machine produces the same tokens in every run, which keeps
  [`compare`](compare.md) working; redacted runs only compare with redacted runs. Delete the file to rotate the tokens.
- **Caveat:** masking covers the columns of the built-in rules. A custom rule loaded with `--rules-dir` that uses other
  column names, or free text, is not masked. Review the file before sharing it.

Nothing is uploaded either way. The HTML shows a "redacted" badge and the JSON has `redacted: true`.

## 6. Use it in CI or a scheduled job

`check` exits with a code a pipeline can act on. By default findings never fail the run; set a policy:

| Code | Meaning |
|---|---|
| `0` | Run finished and the policy passed |
| `1` | A finding at or above `--fail-on` (`critical`, `high`, `medium`, `low`) |
| `2` | Error: connection, configuration or execution. Never reported as a database finding |
| `3` | Coverage below `--min-coverage` (percentage) |

A finding wins over low coverage. The JSON report is next to the HTML, with the same name.

```yaml
# GitHub Actions
- name: codeDB Lens
  env:
    CODEDB_LENS_PASSWORD: ${{ secrets.LENS_PASSWORD }}
    CODEDB_LENS_NO_UPDATE: "1"
  run: >
    codedb-lens check --engine pg --server db.internal --user codedb_lens
    --out lens-report --fail-on high --min-coverage 80
```

Running Lens on a schedule to keep an eye on production? That is what [codeDB](https://codedb.com.br/?utm_source=lens&utm_medium=docs&utm_campaign=ci)
does around the clock, with history and alerts.
