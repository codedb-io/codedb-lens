# MSSQL-BACKUP-001 · Missing or old backups

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Critical | Confirmed | `db_datareader` on `msdb` | SQL Server 2017+, Azure SQL MI, RDS (not Azure SQL Database) |

## What it checks

For every online database (except `tempdb` and snapshots), the latest backup recorded in `msdb.dbo.backupset`:

- **No full backup** ever recorded.
- **Full and differential older than 8 days** (`full_max_hours: 192`).
- **Log backup older than 24 hours** (`log_max_hours: 24`) for databases in `FULL` or `BULK_LOGGED` recovery.

Copy-only backups are ignored.

## Why it matters

Without a recent full backup, a failure loses everything since the last one. In `FULL` recovery without log backups,
point-in-time recovery is impossible and the log keeps growing.

## Next action

```sql
-- See recent history for one database
SELECT TOP (20) database_name, type, backup_finish_date
FROM msdb.dbo.backupset WHERE database_name = N'<db>' ORDER BY backup_finish_date DESC;
```

Check the backup jobs (SQL Agent, Ola Hallengren, your backup tool) and run a manual backup now.

## How to verify

After the backup finishes, confirm it in msdb: `SELECT database_name, type, MAX(backup_finish_date) FROM msdb.dbo.backupset GROUP BY database_name, type`. Each database must show a recent D (full) and, in FULL recovery, an L (log) newer than 24 hours. Also run `RESTORE VERIFYONLY` on the file.

## Limitations

- Backups taken by tools that don't write to `msdb` history are invisible. If that is your case, the finding is a false positive.
- `msdb` history may have been purged (`sp_delete_backuphistory`).
