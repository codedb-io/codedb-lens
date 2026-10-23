# MSSQL-REC-BLOCKING-001 · Blocking of 30 s or more during the recording

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Confirmed | `VIEW SERVER STATE` | Recordings (`codedb-lens record`): SQL Server 2017+, Azure SQL MI, RDS |

## What it checks

The `locks` collector (blocking chains from `sys.dm_exec_requests`, every 15 s). One episode per blocking session while a blocked session waits at least 30 s (`min_wait_s`).

## Why it matters

Sessions waited at least 30 seconds for another session to release its locks. Users see timeouts and frozen screens while it lasts.

## Next action

Start with the blocker of the longest episode. A blocker in "sleeping" status with open transactions is usually an application that forgot to commit; one that is running is a long query or batch to tune or move off-peak.

## How to verify

Record again during the same hours after the fix. The episodes for that blocker must be gone or much shorter.

## Limitations

Samples are taken every 15 seconds, so blocking shorter than that may be missed, and an episode's length is accurate to about 15 seconds.
