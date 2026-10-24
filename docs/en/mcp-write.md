# AI write mode (MCP)

By default the MCP server (`codedb-lens mcp`) only reads: no tool changes a database. Write mode adds one tool,
`execute_sql`, that lets your AI client run `UPDATE`, `INSERT`, `CREATE INDEX` and similar statements **after you
confirm each one**. It is off unless you turn it on, on purpose, in two places.

> **Use it on databases you can afford to get wrong.** The AI writes the SQL. You see it before it runs, but a
> confirmed statement does what it says. Start on a test database, use a login that can only touch what it must, and
> keep backups.

## How it is protected

- **Two keys.** The tool exists only when the profile allows writing **and** the client starts the Lens with
  `--allow-write`. Missing either one, `execute_sql` is not even listed.
- **You confirm every call.** Your AI client shows the SQL, the profile, the database and whether it is a preview, and
  asks you. Nothing runs if you decline or close the prompt.
- **Preview first.** `dry_run` is `true` by default: the statement runs in a transaction, the AI gets the number of
  affected rows, and the transaction is rolled back. To apply it, the AI calls again with `dry_run: false`, and you
  confirm again.
- **No preview for what a transaction cannot hold:** `VACUUM`, `CREATE INDEX CONCURRENTLY`, `ALTER DATABASE`,
  `BACKUP`, `RESTORE`, `DBCC`. These need `dry_run: false` and your confirmation, and cannot be undone.
- **Always refused**, even with write mode on: `DROP DATABASE`, `SHUTDOWN`, server settings (`sp_configure`,
  `ALTER SYSTEM`, `RECONFIGURE`), logins, roles, users and permissions (`CREATE/ALTER/DROP LOGIN|ROLE|USER`, `GRANT`,
  `REVOKE`, `DENY`), operating system and file access (`xp_cmdshell`, `COPY ... PROGRAM`, `lo_import`, `pg_read_file`,
  `OPENROWSET`, `BULK INSERT`), code that runs outside the database (CLR assemblies, SQL Agent jobs, linked and
  foreign servers, external scripts, extensions and untrusted languages), changes of identity, trust or ownership
  (`EXECUTE AS`, `SET ROLE`, `SECURITY DEFINER`, `TRUSTWORTHY`, `ALTER AUTHORIZATION`, `OWNER TO`), transaction
  control (`COMMIT`, `ROLLBACK`: they would turn a preview into a change) and dynamic SQL that would hide any of these
  (`EXEC('...')`, `sp_executesql`, PostgreSQL `DO` and `EXECUTE`). The check reads the whole text, comments and strings
  included, so a word in a comment is refused too: run that one in your own SQL client.
- **Audit log.** Every attempt (preview, change, refusal, declined) is a line in `mcp-audit.jsonl`, next to
  `profiles.json` (`%APPDATA%\codeDB Lens\` on Windows, `~/.config/codedb-lens/` elsewhere): time, profile, database,
  the SQL and its SHA-256, mode, result, affected rows and whether it was rolled back. A run is logged before it
  starts and again with its result; if the log cannot be written, nothing runs.
- **Your login is the real limit.** Use a separate profile with a login made for this. The Lens's own minimal login
  ([Permissions](permissions.md)) cannot write, and should stay that way. A stored procedure the AI calls runs as its
  own code says: if it commits, a preview cannot undo it.
- **Turning it on takes a person at a terminal**, but the Lens cannot stop a program running as you from editing
  `profiles.json` or the client's settings. An AI client that can run commands on your computer can do that too, so
  keep write mode away from clients you let run commands unattended.

## Turn it on

### 1. A login for writing

**SQL Server:**

```sql
CREATE LOGIN lens_ai_write WITH PASSWORD = '<password>';
USE app;
CREATE USER lens_ai_write FOR LOGIN lens_ai_write;
ALTER ROLE db_datareader ADD MEMBER lens_ai_write;
ALTER ROLE db_datawriter ADD MEMBER lens_ai_write;
-- only if the AI should create indexes:
ALTER ROLE db_ddladmin ADD MEMBER lens_ai_write;
```

**PostgreSQL:**

```sql
CREATE ROLE lens_ai_write LOGIN PASSWORD '<password>';
GRANT CONNECT ON DATABASE app TO lens_ai_write;
GRANT USAGE ON SCHEMA public TO lens_ai_write;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO lens_ai_write;
```

Save a profile with it in the app (for example `app-write`), separate from your diagnosis profile.

### 2. Allow writing on that profile

```bash
codedb-lens mcp allow-write --profile app-write
```

It asks you to type the profile name. The app then shows a warning on the **AI (MCP)** screen while any profile allows
writing.

### 3. Add `--allow-write` to the client

Edit the client's settings by hand (`codedb-lens mcp install` never adds it, and running it again removes it) and add
`--allow-write` to the Lens's arguments, preferably limited to that profile:

```json
{
  "mcpServers": {
    "codedb-lens": {
      "command": "C:\\Users\\you\\AppData\\Local\\Programs\\codeDB Lens\\codedb-lens.exe",
      "args": ["mcp", "--profile", "app-write", "--allow-write"]
    }
  }
}
```

Restart the client. `list_profiles` now shows `write_enabled: true` for that profile.

Your client must be able to ask you to confirm (MCP *elicitation*); if it cannot, every write is refused.
`--allow-write=no-confirm` lets such a client write with only its own per-tool approval, if it has one. Avoid it.

## Turn it off

```bash
codedb-lens mcp revoke-write --profile app-write
```

It takes effect on the next call, without restarting the client. Remove `--allow-write` from the client's settings too.
`codedb-lens mcp status` lists the profiles that still allow writing.
