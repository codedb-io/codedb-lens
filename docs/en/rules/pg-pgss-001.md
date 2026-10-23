# PG-PGSS-001 · pg_stat_statements not available

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Confirmed | `pg_monitor` (`pg_read_all_settings`) | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

Whether `pg_stat_statements` is in `shared_preload_libraries` **and** the extension exists in the database Lens connects to.

## Why it matters

It's the only built-in record of which queries consume time, I/O and calls. Without it, finding the query to tune is guesswork,
and Lens's query rules lose coverage.

## Next action

```ini
# postgresql.conf (restart required)
shared_preload_libraries = 'pg_stat_statements'
```

```sql
CREATE EXTENSION pg_stat_statements;
```

On RDS/Aurora use the parameter group; on Azure Flexible Server, the `shared_preload_libraries` server parameter. Both already
preload it by default on recent versions; only `CREATE EXTENSION` is needed.

## How to verify

After the restart and `CREATE EXTENSION`, run `SELECT count(*) FROM pg_stat_statements`: it must work without error and start returning rows as queries run. `SHOW shared_preload_libraries` must list `pg_stat_statements`. Give it some hours of load before reading the top queries, since it only counts from the moment it is enabled.

## Limitations

Only the database Lens connects to is checked for the extension.
