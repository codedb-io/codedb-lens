# MSSQL-CHECKDB-001 · No recent clean DBCC CHECKDB

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Confirmed | `VIEW ANY DEFINITION` | SQL Server 2017+, Azure SQL MI, RDS (not Azure SQL Database) |

## What it checks

`DATABASEPROPERTYEX(name, 'LastGoodCheckDbTime')` for every online database except `tempdb` and snapshots.
A finding is raised when the last clean run is older than 7 days (`max_days: 7`) or never happened.
Databases created within the window are skipped.

## Why it matters

Corruption doesn't announce itself. If it goes undetected longer than your backup retention, every backup carries it
and there's nothing clean left to restore.

## Next action

```sql
DBCC CHECKDB (N'<db>') WITH NO_INFOMSGS, ALL_ERRORMSGS;
```

Schedule it weekly (Ola Hallengren's `DatabaseIntegrityCheck` is a common choice). For large databases with short
windows, run it on a restored copy of the backup.

## How to verify

After DBCC CHECKDB completes without errors, confirm the date: `SELECT DATABASEPROPERTYEX(N'db', 'LastGoodCheckDbTime')` must be today. Then check that the schedule really runs: the value should renew at least once a week.

## Limitations

- `LastGoodCheckDbTime` only records runs that finished without errors: a run that found corruption doesn't update it.
- Checks run on another server against restored copies are invisible here.
