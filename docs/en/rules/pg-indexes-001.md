# PG-INDEXES-001 · Unused, duplicate or invalid indexes

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Low | Confirmed (invalid, duplicate) · Hint (unused) | `pg_monitor` + `CONNECT` on each database | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

Runs **in each database**, on `pg_index` and `pg_stat_user_indexes`:

- **Invalid:** `indisvalid = false`, usually left by a failed `CREATE INDEX CONCURRENTLY`.
- **Unused:** zero scans, not unique or primary key, at least 10 MB (`min_unused_mb`).
- **Duplicate:** same columns, operator classes, expressions and predicate as another index on the table (a primary key
  is never reported as the duplicate).

## Why it matters

Every index is written on every insert and on non-HOT updates, takes cache memory and backup space, and is rebuilt by
maintenance. Invalid indexes are maintained but never used for reads.

## Next action

```sql
DROP INDEX CONCURRENTLY <index>;   -- invalid or duplicate
```

For unused indexes, confirm over a full business cycle and check `pg_stat_user_indexes` on each replica before dropping.

## How to verify

After dropping, confirm the index is gone: `SELECT indexrelid::regclass FROM pg_index WHERE NOT indisvalid` must return nothing, and `SELECT relname, indexrelname, idx_scan FROM pg_stat_user_indexes ORDER BY idx_scan` should no longer list the dropped ones. Watch query times and the slow log for a few days; if one regresses, recreate the index. Counters reset with `pg_stat_reset()`, so judge usage over a full business cycle.

## Limitations

Usage counters reset with `pg_stat_reset()`; replicas keep their own counters.
