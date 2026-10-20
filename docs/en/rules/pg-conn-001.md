# PG-CONN-001 · Connections close to the limit, or too many idle

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Hint · Confirmed at 95% of `max_connections` | `pg_monitor` (`pg_read_all_stats`) | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

Client backends in `pg_stat_activity`:

- **Near the limit:** 80% or more of `max_connections` in use (`hint_pct`; `confirmed_pct: 95`).
- **No pooler:** at least 50 idle connections (`idle_min`) making up 70% or more of all connections (`idle_pct`).

## Why it matters

At `max_connections`, new connections fail and the application goes down. Each backend is a process with its own memory;
hundreds of idle ones waste RAM and slow down snapshots for everyone.

## Next action

Use a pooler (PgBouncer in transaction mode, RDS Proxy, or a right-sized application pool) rather than raising
`max_connections`. Also set `idle_session_timeout` for clients that leave connections open.

## Limitations

A snapshot sees the current moment; connection peaks usually happen at specific times of day.
