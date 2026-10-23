# PG-REC-CONN-001 · Connections near max_connections

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Confirmed | `pg_monitor` (`pg_read_all_stats`) | Recordings (`codedb-lens record`): PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

The `connections` collector: client connections as a percentage of `max_connections` minus `superuser_reserved_connections`. An episode is two or more samples in a row at 90% or more (`max_pct`).

## Why it matters

Client connections used 90% or more of the usable slots for at least 30 seconds. When they run out, new connections fail with "too many clients" and the application stops.

## Next action

Look at how many of those connections were idle or idle in transaction. Many idle connections call for a pooler (PgBouncer) or smaller application pools, not a higher max_connections.

## How to verify

Record again during the same hours after the change. Connections must stay well below max_connections.

## Limitations

The limit shown is max_connections minus superuser_reserved_connections; on PostgreSQL 16+ reserved_connections also takes slots and is not subtracted.
