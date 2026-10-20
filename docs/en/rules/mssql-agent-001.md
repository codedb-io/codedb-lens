# MSSQL-AGENT-001 · SQL Agent jobs failing

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Confirmed | `SQLAgentReaderRole` on `msdb` | SQL Server 2017+ (not Express), Azure SQL MI, RDS |

## What it checks

Enabled jobs in `msdb.dbo.sysjobs` whose **last** outcome (`sysjobhistory`, `step_id = 0`) failed or was canceled
within the last 7 days (`days: 7`). The evidence also shows how many times each job failed in that window.

## Why it matters

Most jobs exist to keep something running: backups, index maintenance, ETL. A failing job is often the first sign
that one of those silently stopped.

## Next action

```sql
EXEC msdb.dbo.sp_help_jobhistory @job_name = N'<job>', @mode = N'FULL';
```

Fix the failing step and configure an operator with `@notify_level_email = 2` (on failure).

## Limitations

- A job that recovered (last run succeeded) is not reported, even if it failed earlier in the window.
- Job history is capped by the Agent's history retention settings.
- Not available on Express or Azure SQL Database, which have no SQL Agent.
