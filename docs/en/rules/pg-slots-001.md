# PG-SLOTS-001 · Replication slots retaining WAL, or replicas lagging

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Hint · Confirmed above 10 GB retained or `wal_status` lost/unreserved | `pg_monitor` | PostgreSQL 14+, RDS, Aurora, Azure Flexible (on the primary) |

## What it checks

- `pg_replication_slots`: **inactive** slots retaining at least 1 GB of WAL (`retained_mb`), or any slot whose
  `wal_status` is `unreserved` or `lost`.
- `pg_stat_replication`: replicas whose `replay_lag` is at least 60 s (`lag_s`).

## Why it matters

A slot keeps every WAL segment its consumer hasn't confirmed. If the consumer is gone (an old replica, a CDC tool that was
turned off), WAL piles up until the disk fills and the primary stops accepting writes.

## Next action

```sql
-- slot no longer needed
SELECT pg_drop_replication_slot('<slot_name>');
```

For lagging replicas, check the replica's disk and CPU, the network, and long queries on the replica that delay replay
(`max_standby_streaming_delay`). Consider `max_slot_wal_keep_size` as a safety limit.

## Limitations

Only meaningful on the primary. A slot may be inactive for a few seconds while its consumer reconnects.
