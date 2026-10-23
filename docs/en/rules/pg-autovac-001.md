# PG-AUTOVAC-001 · Many dead rows (autovacuum falling behind)

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Hint · Confirmed at 50% dead | `pg_monitor` + `CONNECT` on each database | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

Runs **in each database**. Tables in `pg_stat_user_tables` with at least 10,000 dead rows (`min_dead`) making up at least
20% of the table (`dead_pct`), with the last vacuum and analyze times. Top 20.

## Why it matters

Dead rows are versions left behind by updates and deletes. Until vacuum removes them, scans read them anyway and the
space can't be reused, so tables and indexes grow (bloat).

## Next action

1. Check what blocks vacuum: old transactions (`PG-LONGTX-001`), abandoned slots (`PG-SLOTS-001`), old prepared transactions.
2. For hot tables, make autovacuum trigger earlier:

```sql
ALTER TABLE <table> SET (autovacuum_vacuum_scale_factor = 0.02, autovacuum_vacuum_threshold = 1000);
```

3. Raise `autovacuum_vacuum_cost_limit` if autovacuum runs but never keeps up.

## How to verify

Check the table: `SELECT n_dead_tup, last_autovacuum, last_vacuum FROM pg_stat_user_tables WHERE relname = 'table'`. After the vacuum, `n_dead_tup` must drop sharply and `last_autovacuum` or `last_vacuum` must be recent. Then watch it for a day: if dead rows climb back above the threshold, autovacuum still can't keep up and the cause (old transactions, slots, settings) remains.

## Limitations

Counters reset with `pg_stat_reset()` and after a crash. A table with `autovacuum_enabled = false` is reported like any other.
