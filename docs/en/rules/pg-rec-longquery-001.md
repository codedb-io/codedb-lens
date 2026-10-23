# PG-REC-LONGQUERY-001 · Queries running for 1 minute or more

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Confirmed | `pg_monitor` (`pg_read_all_stats`) | Recordings (`codedb-lens record`): PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

The `activity` collector (`pg_stat_activity` in state `active`, every 15 s). One episode per backend and query while the statement has been running at least 60 s (`min_s`).

## Why it matters

Statements ran for at least a minute. Long statements hold locks, keep old row versions alive (autovacuum cannot clean them) and compete with everything else.

## Next action

Separate what is expected (backups, VACUUM, reports at night) from what is not. For the rest, run EXPLAIN (ANALYZE, BUFFERS) on the query (its text is in the report) and consider statement_timeout for the application role.

## How to verify

Record again during the same hours after the change. The query must not appear, or its peak duration must be clearly lower.

## Limitations

Samples are taken every 15 seconds, so the real duration may be up to 15 seconds longer than the peak shown. Maintenance commands show up here too.
