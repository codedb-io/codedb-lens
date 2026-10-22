# MSSQL-TOPQUERY-001 · A single query dominates CPU

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Hint | `VIEW SERVER STATE` | SQL Server 2017+, Azure SQL MI, RDS, Azure SQL Database |

## What it checks

`sys.dm_exec_query_stats` grouped by `query_hash`. Reported when the most expensive query uses at least 40% of the CPU
recorded in the plan cache (`dominant_pct`) and the cache holds at least 60 s of CPU (`min_total_cpu_s`). The evidence
lists the top 5 with share, executions, average CPU, duration and reads.

On Azure SQL Database the plan cache covers only the connected database.

## Why it matters

Tuning a database is rarely tuning everything. When one query dominates, fixing it is the change with the highest return.

## Next action

Get the plan for the top query (`sys.dm_exec_query_plan`, or Query Store if enabled), look for scans, missing indexes,
implicit conversions and bad estimates. Also check whether it runs more often than the application needs.

## Limitations

The plan cache is emptied on restart and under memory pressure; `OPTION (RECOMPILE)` queries may be missing. Without
history you can't tell when the query started dominating.
