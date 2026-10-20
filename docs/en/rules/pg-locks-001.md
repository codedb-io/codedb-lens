# PG-LOCKS-001 · Sessions blocked by locks right now

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Confirmed | `pg_monitor` (`pg_read_all_stats`) | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

Sessions in `pg_stat_activity` waiting on a lock (`wait_event_type = 'Lock'`) for at least 5 seconds (`min_wait_s`), with
the first blocker from `pg_blocking_pids()`: its state, user, application and transaction age.

## Why it matters

Locks queue: a single `ALTER TABLE` waiting behind an open transaction makes every later query on that table wait too.

## Next action

- Blocker `idle in transaction`: an application left a transaction open. Fix the application; set
  `idle_in_transaction_session_timeout` as a safety net.
- Blocker `active`: a long statement. As a last resort, `SELECT pg_cancel_backend(<pid>)` (cancel the query) before
  `pg_terminate_backend(<pid>)` (close the session).
- For DDL in production, use `SET lock_timeout = '3s'` so a migration gives up instead of freezing the table.

## Limitations

A snapshot only sees blocking at the moment of the check.
