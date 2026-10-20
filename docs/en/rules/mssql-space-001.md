# MSSQL-SPACE-001 · Disk or transaction log running out of space

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Hint · Confirmed below 5% free / above 90% log used | `VIEW SERVER STATE` (2022+: `VIEW SERVER PERFORMANCE STATE`) | SQL Server 2017+, Azure SQL MI, RDS |

## What it checks

- **Volumes** that hold database files (`sys.dm_os_volume_stats`) with less than 15% free (`volume_hint_pct`).
- **Transaction logs** more than 75% used (`log_hint_pct`, active log ÷ log size from `sys.dm_db_log_stats`) whose `log_reuse_wait_desc`
  says something is preventing reuse (anything other than `NOTHING` or `CHECKPOINT`).

## Why it matters

A full volume or a full log stops writes in every affected database. A log that can't be reused keeps growing until it hits one of them.

## Next action

Act on `log_reuse_wait_desc`:

| Value | What to do |
|---|---|
| `LOG_BACKUP` | Take log backups (or switch to `SIMPLE` if point-in-time restore isn't needed) |
| `ACTIVE_TRANSACTION` | Find the open transaction: `DBCC OPENTRAN` |
| `REPLICATION` / `AVAILABILITY_REPLICA` | Check the log reader or the secondary replica |

## Limitations

A snapshot tells how full things are, not how fast they grow. Projecting when a volume fills up needs history.
