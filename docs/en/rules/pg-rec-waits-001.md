# PG-REC-WAITS-001 · Wait peaks during the recording

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Hint | `pg_monitor` (`pg_read_all_stats`) | Recordings (`codedb-lens record`): PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

The `activity` collector: active sessions grouped by `wait_event_type` and `wait_event` (waits on the client left out). An episode is two or more samples in a row with 5 or more sessions on the same event (`min_sessions`).

## Why it matters

For at least 30 seconds, several active sessions were waiting on the same event (locks, disk, WAL, internal locks). Each event points to a different bottleneck.

## Next action

Match the peak with the event type: Lock = blocking between sessions, IO (DataFileRead) = reading from disk (missing indexes or memory), LWLock = contention inside the server (WAL, buffers), IO (WALWrite/WALSync) = WAL disk. Then look at the top queries of the same period.

## How to verify

Record again during the same hours after the change. The peaks of that event must be lower or shorter.

## Limitations

PostgreSQL shows only what each session waits on at the moment of the sample, not accumulated wait time, so short waits between samples are missed. A wait peak is a symptom, not a cause, so this is a hint.
