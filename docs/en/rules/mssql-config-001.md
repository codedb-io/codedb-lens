# MSSQL-CONFIG-001 · Instance settings left at defaults

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Hint | `VIEW SERVER STATE` (host CPU and RAM) | SQL Server 2017+, Azure SQL MI, RDS |

## What it checks

| Setting | Flagged when | Suggested starting point |
|---|---|---|
| `max server memory (MB)` | Still at the default (2147483647) | ~80% of host RAM |
| `max degree of parallelism` | `0` on hosts with more than 8 logical CPUs (`maxdop_cpus`) | `8` |
| `cost threshold for parallelism` | Still at the default `5` | `50` |
| `optimize for ad hoc workloads` | `0` | `1` |

## Why it matters

With unlimited memory SQL Server can starve the OS and other services. With the parallelism defaults, cheap queries
go parallel and compete for CPU (`CXPACKET` / `CXCONSUMER` waits).

## Next action

```sql
EXEC sp_configure 'show advanced options', 1; RECONFIGURE;
EXEC sp_configure 'max server memory (MB)', <value>; RECONFIGURE;
```

Change one setting at a time and watch the effect.

## How to verify

Check the value in use, not just the configured one: `SELECT name, value, value_in_use FROM sys.configurations WHERE name IN ('max server memory (MB)', 'max degree of parallelism', 'cost threshold for parallelism', 'optimize for ad hoc workloads')`. `value_in_use` must show the new number (run `RECONFIGURE` if it does not). Then watch CPU and memory for a few days.

## Limitations

These are common starting points, not universal answers. Hosts that run other services (SSIS, SSRS, applications) need
more memory left to the OS.
