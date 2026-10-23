# MSSQL-REC-WORKERS-001 · Worker threads near the limit

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Confirmed | `VIEW SERVER STATE` | Recordings (`codedb-lens record`): SQL Server 2017+, Azure SQL MI, RDS |

## What it checks

The `workers` collector: worker threads in `sys.dm_os_schedulers` as a percentage of `max_workers_count`. An episode is two or more samples in a row at 90% or more (`max_pct`).

## Why it matters

SQL Server used 90% or more of its worker threads for at least 30 seconds. When they run out, new requests wait (THREADPOOL) and even new connections can fail, while the CPU may look idle.

## Next action

The usual cause is blocking (many sessions stuck behind one) or heavy parallelism. Check the blocking and wait findings of the same period first; do not raise max worker threads before finding the cause.

## How to verify

Record again during the same hours after the fix. Workers must stay well below max_workers.

## Limitations

Counts threads created by SQL Server, which stay around for a while after a peak.
