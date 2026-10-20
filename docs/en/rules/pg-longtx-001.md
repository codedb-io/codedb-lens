# PG-LONGTX-001 · Long-running or idle-in-transaction sessions

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Confirmed | `pg_monitor` (`pg_read_all_stats`) | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

Client sessions in `pg_stat_activity` with a transaction open for more than 5 minutes (`max_tx_s: 300`), or in
`idle in transaction` for more than 60 seconds (`idle_tx_s: 60`). Shows who (user, application, client address) and the last query.

## Why it matters

The oldest open transaction sets the horizon for vacuum **across the whole cluster**: dead rows newer than it can't be
removed, so tables and indexes bloat. It may also hold locks that block other sessions.

## Next action

Find the application behind each `pid`. `idle in transaction` almost always means a missing `COMMIT` or a connection
returned to the pool mid-transaction. As a safety net:

```sql
ALTER SYSTEM SET idle_in_transaction_session_timeout = '10min';
SELECT pg_reload_conf();
```

## Limitations

A snapshot only sees transactions open at the moment of the check.
