# PG-REC-BLOCKING-001 · Lock waits of 30 s or more during the recording

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Confirmed | `pg_monitor` (`pg_read_all_stats`) | Recordings (`codedb-lens record`): PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

The `locks` collector (`pg_stat_activity` + `pg_blocking_pids()`, every 15 s). One episode per blocking backend while a session waits on a lock at least 30 s (`min_wait_s`).

## Why it matters

Sessions waited at least 30 seconds for a lock held by another backend. Users see timeouts and frozen screens while it lasts.

## Next action

Start with the blocker of the longest episode. A blocker "idle in transaction" is an application that opened a transaction and did not finish it; set idle_in_transaction_session_timeout and fix the application. An active blocker is a long statement to tune or move off-peak.

## How to verify

Record again during the same hours after the fix. The episodes for that blocker must be gone or much shorter.

## Limitations

Samples are taken every 15 seconds, so lock waits shorter than that may be missed, and an episode's length is accurate to about 15 seconds.
