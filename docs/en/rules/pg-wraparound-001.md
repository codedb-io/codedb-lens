# PG-WRAPAROUND-001 · Transaction ID wraparound risk

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Critical | Hint (age ≥ 1.5 × `autovacuum_freeze_max_age`) · Confirmed (age ≥ 1 billion) | `public` | All PostgreSQL versions and services |

## What it checks

`age(datfrozenxid)` of every database that accepts connections.

## Why it matters

Near 2³¹ (≈ 2.1 billion) transaction IDs, PostgreSQL stops accepting writes to protect data. Recovery requires a long VACUUM.
An age well above `autovacuum_freeze_max_age` means autovacuum is falling behind.

## Next action

```sql
-- In the affected database: the oldest tables
SELECT c.oid::regclass, age(c.relfrozenxid) FROM pg_class c
WHERE c.relkind IN ('r', 'm', 't') ORDER BY 2 DESC LIMIT 20;
```

Run `VACUUM (FREEZE, VERBOSE)` on them and look for what blocks autovacuum: long transactions,
abandoned replication slots, old prepared transactions.

## How to verify

Run `SELECT datname, age(datfrozenxid) FROM pg_database ORDER BY 2 DESC`: once VACUUM (FREEZE) finishes, the age must be well below `autovacuum_freeze_max_age` (default 200 million). Check again after a day: if autovacuum keeps it low, what held it back (old transactions, slots, prepared transactions) is gone.

## Limitations

A snapshot shows the current age, not the consumption rate. How many days are left needs history.
