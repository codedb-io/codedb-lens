# MSSQL-REC-WAITS-001 · Wait peaks during the recording

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Hint | `VIEW SERVER STATE` | Recordings (`codedb-lens record`): SQL Server 2017+, Azure SQL MI, RDS |

## What it checks

The `waits` collector: wait time per wait type from `sys.dm_os_wait_stats`, as the difference between samples, divided by the seconds between them. An episode is two or more samples in a row at 2000 ms of waiting per second or more (`min_wait_ms_per_s`).

## Why it matters

For at least 30 seconds, queries spent a lot of time waiting on one resource (disk, locks, CPU, memory, log). Each wait type points to a different bottleneck.

## Next action

Match the peak with the wait type: PAGEIOLATCH = reading data from disk (missing indexes or memory), WRITELOG = log disk, LCK_M = blocking, SOS_SCHEDULER_YIELD = CPU, RESOURCE_SEMAPHORE = memory grants, CXPACKET/CXCONSUMER = parallelism. Then look at the top queries of the same period.

## How to verify

Record again during the same hours after the change. The peaks of that wait type must be lower or shorter.

## Limitations

A wait peak is a symptom, not a cause, so this is a hint. Only waits with a clear meaning are recorded; idle and background waits are left out.
