# MSSQL-BLOCKING-001 · Sessions blocked right now

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Confirmed | `VIEW SERVER STATE` | SQL Server 2017+, Azure SQL MI, RDS, Azure SQL Database |

## What it checks

`sys.dm_exec_requests` with a `blocking_session_id`, blocked for at least 5 seconds (`min_wait_ms: 5000`), with the
blocker's status, login, host, program and open transactions from `sys.dm_exec_sessions`.

## Why it matters

Blocked sessions sit idle until the blocker releases its locks. Users see timeouts; long chains can exhaust worker
threads (`THREADPOOL`).

## Next action

- Blocker **sleeping** with `blocker_open_trans > 0`: an application opened a transaction and didn't commit. Fix the
  application; as an emergency, talk to the owner before `KILL <session>`.
- Blocker **running**: a long query or batch. Check its plan and whether it can run off-peak.

## How to verify

Re-check the blocking chain: `SELECT session_id, blocking_session_id, wait_time FROM sys.dm_exec_requests WHERE blocking_session_id <> 0`. It must come back empty, or with waits well below the threshold. One check is not enough: repeat it during the busy hours, since blocking comes and goes.

## Limitations

A snapshot only sees what is blocked at the moment of the check. Blocking that happened at 03:00 is invisible here.
