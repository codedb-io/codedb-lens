# Use with AI (MCP)

Ask Claude, GitHub Copilot or Cursor about your SQL Server or PostgreSQL in plain words. `codedb-lens mcp` is a
[Model Context Protocol](https://modelcontextprotocol.io/) server: your AI client starts it on your computer and
calls the Lens's tools. **It only reads.** No port is opened and nothing is installed on the server.

Two uses, decided by the login of the profile you give the AI:

- **Diagnosis**: the Lens's minimal login ([Permissions](permissions.md)). The AI runs the health check, reads
  recordings and explains what to fix. It cannot read business data.
- **Data**: a login that reads the tables you choose ([Use with data](mcp-data.md)). The AI lists tables, describes
  them and runs `SELECT`s to answer questions, compare periods or build tables and charts in the chat.

> **Privacy.** The Lens makes no network call. Your AI client sends what the Lens returns (findings, and data rows
> when the login reads tables) to the model's provider. Mask names with `--redact`, and give the AI a login without
> access to sensitive columns.

## 1. Before you start

- codeDB Lens installed, by any channel ([Getting started](getting-started.md)).
- A saved connection profile with its password: create it in the app, or with `codedb-lens check ... --save NAME`.
  The AI never sees passwords.
- For a Linux server or CI without profiles, see [MCP with Docker](mcp-docker.md).

## 2. Add the Lens to your client

The app shows these commands, ready to copy, on the **AI (MCP)** screen. The installer does not put the Lens on
`PATH` by default, so the commands use its full path:

```bash
"%LOCALAPPDATA%\Programs\codeDB Lens\codedb-lens.exe" mcp install --client claude-desktop --profile prod-diag
```

| Client | `--client` | What happens |
|---|---|---|
| Claude Desktop | `claude-desktop` | Adds the Lens to `claude_desktop_config.json` |
| VS Code (Copilot) | `vscode` | Adds the Lens to your user `mcp.json` |
| Cursor | `cursor` | Adds the Lens to `~/.cursor/mcp.json` |
| Claude Code | `claude-code` | Prints the `claude mcp add` command to run; Claude Code manages its own settings |

`mcp install` changes only the `codedb-lens` entry, keeps everything else, saves the previous file as `.bak` and does
nothing on a second run. `--dry-run` shows the change without writing it. If the file is not valid JSON, it is left
alone and the command prints the snippet to paste by hand. **Restart the client** afterwards.

Options:

- `--profile a,b`: the profiles the AI may use (default: all saved profiles). Prefer naming them.
- `--redact`: mask host, database, object, login and file names in check, compare and recording results.

`codedb-lens mcp status` shows which clients have the Lens and whether the recorded path still exists.
`mcp uninstall --client ...` removes the entry.

## 3. Ask

- "Run the diagnosis on prod-diag and tell me what to fix first."
- "What caused yesterday's slowdown at 2 pm? Use the recording."
- "Which tables are in SalesDB?"
- "Top 10 customers by revenue this quarter."
- "Compare this month's orders with the same month last year."

The client also lists three prompts: **diagnose**, **incident** and **explore_data**.

## Tools

| Tool | What it does |
|---|---|
| `list_profiles` | Profiles the AI may use: name, engine, host, initial database. Never passwords or connection strings |
| `check` | Runs the health check (10–60 s): grade with coverage, findings with evidence; saves the report in the profile's history, like the app |
| `get_finding` | One rule of a saved run, with all its evidence rows |
| `history` / `compare` | Saved runs; new, resolved and persisting findings between two runs (compare needs the [free key](compare.md)) |
| `list_rules` / `explain_rule` | The rule catalog: what each rule measures, its SQL and its documentation |
| `list_recordings` / `recording_report` | Recordings and a report of a period ([Record over time](record.md)) |
| `record_command` | The `codedb-lens record` command to run in a terminal: a recording outlives the chat, so the AI never starts one |
| `list_databases` / `list_tables` / `describe_table` | What the login can see: databases, tables and views, columns, keys and indexes |
| `query` | One read-only `SELECT`, with limits |

## Limits

`query` runs one `SELECT` (or `WITH ... SELECT`) and refuses anything else: writes, `EXEC`, several statements, `SELECT
INTO`, functions with side effects. It runs in a transaction that is always rolled back (PostgreSQL also in a
read-only one), stops at the profile's row limit (500 by default, cells cut at 2 KB) and at its time limit (10 s).
A cut result says `truncated: true`, and the AI is told to refine its query. Change the limits per connection in the
app (**Advanced connection** → *Max rows for the AI*, *Max query time*), up to 10,000 rows and 120 s.

On SQL Server, `query` reads uncommitted data, like the health check, so it never waits for writers.

## Common problems

| Problem | What to do |
|---|---|
| The client does not show the Lens's tools | Restart the client; run `codedb-lens mcp status` |
| It stopped working after an update | The path changed: run the `mcp install` command again |
| "no saved password for profile" | Save the password in the app, or set `CODEDB_LENS_PASSWORD` in the client's server settings |
| "the login of profile ... cannot read this object" | The profile's login has no `SELECT` on that table: grant it, or use another profile |
| "profile ... not available to this AI session" | It is not in `--profile`: add it and run `mcp install` again |
