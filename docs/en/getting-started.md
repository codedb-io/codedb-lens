# Getting started

## 1. Create a read-only login

Use the scripts in [Minimum permissions](permissions.md). Lens never writes to your database, and the login doesn't need
more than that.

## 2. Install

Download the files from [GitHub Releases](https://github.com/codedb-io/codedb-lens/releases/latest).

- **Windows:** run `codedb-lens_<version>_windows_amd64_setup.exe`. It installs for your user only (no admin) and adds
  **codeDB Lens** to the Start menu. A portable `.zip` is also available. The installer is not code-signed yet, so
  Windows SmartScreen may warn: choose **More info → Run anyway**.
- **Linux (x64 / arm64):** extract the `.tar.gz` and put `codedb-lens` in your `PATH`.

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
`--sslmode` (PostgreSQL), `--no-update-check`.

## 4. Read the report

Reports are saved as HTML and JSON in `Documents\codeDB Lens\reports\<profile>` (Windows) or
`~/codedb-lens/reports/<profile>` (Linux). See [how to read the grade and coverage](index.md#how-to-read-the-report).
Each finding links to its rule page with the reasoning, the limits and what to do next.

## 5. Use it in CI or a scheduled job

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
