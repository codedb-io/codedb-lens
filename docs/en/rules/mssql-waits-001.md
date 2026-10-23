# MSSQL-WAITS-001 · Instance waiting a lot on one resource

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Hint · Confirmed for `THREADPOOL` / `RESOURCE_SEMAPHORE` | `VIEW SERVER STATE` (2022+: `VIEW SERVER PERFORMANCE STATE`) | SQL Server 2017+, Azure SQL MI, RDS, Azure SQL Database |

## What it checks

`sys.dm_os_wait_stats`, counting only waits with a clear meaning (idle and background waits are never counted), grouped
by what they mean. A category is reported when it is at least 20% of those waits (`min_share_pct`) **and** the instance
spent at least 5 minutes waiting on it per hour of uptime (`min_wait_s_per_hour: 300`).

On Azure SQL Database it reads `sys.dm_db_wait_stats` (this database only) and also counts the service tier's log write limit (`*LOG_RATE_GOVERNOR`): if it dominates, the tier is too small for the write load.

## What each wait usually means

| Meaning | Wait types | Usual causes |
|---|---|---|
| Disk: reading data pages | `PAGEIOLATCH_*` | Too little memory for the working set, missing indexes causing scans, slow storage |
| Disk: transaction log writes | `WRITELOG` | Slow log disk, many tiny transactions (commit per row) |
| Blocking | `LCK_M_*` | Long transactions, missing indexes on updates, isolation level |
| Page contention in memory | `PAGELATCH_*` | TempDB allocation contention, last-page insert hotspots |
| CPU pressure | `SOS_SCHEDULER_YIELD` | Expensive queries, too few CPUs |
| Parallelism | `CXPACKET`, `CXCONSUMER` | MAXDOP / cost threshold at defaults, skewed parallel plans |
| Memory grants | `RESOURCE_SEMAPHORE` | Queries asking for huge sort/hash memory, bad estimates |
| Out of worker threads | `THREADPOOL` | Blocking chains, too many parallel queries. Serious: new connections may fail |
| Application reading slowly | `ASYNC_NETWORK_IO` | Application processing rows one by one, or the network |

## Next action

Start from the top category and its usual causes. Compare with `MSSQL-TOPQUERY-001` and `MSSQL-BLOCKING-001` in the same report.

## How to verify

Waits are cumulative since the last restart, so a fix only shows as a change over time. Sample twice, some hours apart, and compare: `SELECT wait_type, wait_time_ms FROM sys.dm_os_wait_stats WHERE wait_type = N'the_wait'`. The growth per hour of that wait must be clearly lower than before the change. On a test instance you can zero the counters with `DBCC SQLPERF('sys.dm_os_wait_stats', CLEAR)`.

## Limitations

Totals accumulate since the restart. They show the long-term profile, not what is happening now; only the difference
between two moments shows the present.
