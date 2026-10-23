# Record over time

`check` is a snapshot: it sees blocking or a slow query only if it is happening at that moment. `record` watches the
instance for a period, every 15 seconds, and `report` turns the recording into a report of what happened: blocking
episodes, wait peaks, long queries, connections near the limit and the heaviest queries of the period.

- **1 hour** without a key, **24 hours** with the [free key](compare.md#get-the-free-key).
- Same read-only login and permissions as `check` ([Permissions](permissions.md)). Nothing is installed or created on
  the server: no Extended Events session, no table, no job.
- Everything stays on your machine, in the recording folder.

## Record

**App:** select a connection, open the **Record** tab, pick 1 or 24 hours and click **Start recording**. Keep the app
open: closing it stops the recording (what was recorded is kept). Windows does not go to sleep while it records.

**Command line:**

```bash
codedb-lens record --engine mssql --server prod01 --auth windows --duration 24h
codedb-lens record --engine pg --dsn-env PG_DSN --duration 1h --out ./night-of-02-11
```

The connection flags are the same as `check`. Without `--out`, the folder is `./codedb-lens-record-<host>-<date>`.
Progress is printed once a minute. **Ctrl+C** stops cleanly, and so does `SIGTERM` (`docker stop`, systemd).

The recording also stops by itself at the end of the duration, or if it reaches **500 MB** on disk (a 24-hour recording
usually stays far below that). Only one recording runs at a time on a computer.

If the connection drops, the recording keeps going and reconnects by itself (after 15 s, 30 s, then every minute). The
time without data appears as **No data** in the report, never as OK. The deadline does not move.

### On a server, with Docker

A recording of 24 hours is easier on a machine that stays on. The image has no shell and writes only to the folder
you mount:

```bash
docker run -d --name lens-rec -e CODEDB_LENS_PASSWORD -v "$PWD:/out" \
  ghcr.io/codedb-io/codedb-lens record --engine pg --server db01 --user codedb_lens --duration 1h --out /out/rec
docker stop lens-rec    # stops early, cleanly
```

The free key lives in the computer's credential store, so inside a container `record` runs for up to 1 hour. For 24
hours, use the binary on the host (below) with the key imported there.

### On a Linux server, with systemd

```ini
# /etc/systemd/system/lens-record.service
[Service]
Type=simple
User=lens
Environment=CODEDB_LENS_PASSWORD=change-me
ExecStart=/usr/bin/codedb-lens record --engine mssql --server db01 --user codedb_lens --duration 24h --out /var/lib/lens/rec-%i
```

`systemctl start lens-record` starts it; `systemctl stop lens-record` stops it cleanly. In the app, **Copy command for
the server** gives the `record` line for the selected connection, without the password.

## Report

```bash
codedb-lens report ./codedb-lens-record-prod01-20261102-2200
codedb-lens report ./codedb-lens-record-prod01-20261102-2200 --from 02:00 --to 03:30
```

It writes `report.html` and `report.json` in the recording folder (`report-<from>_<to>` for a period). It works on a
recording that is still running or that was interrupted. In the app, click a recording in the list, or **View report**
while it records.

- `--from` / `--to`: `15:04` (the first such time after the recording started, so `02:00` in a night recording is the
  next day), `2006-01-02 15:04` or RFC 3339. Times are in this computer's zone.
- `--redact`: masks host, database, login and application names and drops query texts, like `check --redact`.
- `--lang en|pt-BR`. The report carries both languages; the button at the top switches.

### What the report shows

- **Time with data**: share of the period with samples. It is the coverage seal of a recording: below 100% means the
  connection dropped, the computer slept or the recording was paused.
- **Findings** of the series rules, each row an episode (start, end, samples, peak and the sample at the peak, with
  the query text when there is one):

| SQL Server | PostgreSQL | Finding |
|---|---|---|
| [MSSQL-REC-BLOCKING-001](rules/mssql-rec-blocking-001.md) | [PG-REC-BLOCKING-001](rules/pg-rec-blocking-001.md) | blocking of 30 s or more |
| [MSSQL-REC-WAITS-001](rules/mssql-rec-waits-001.md) | [PG-REC-WAITS-001](rules/pg-rec-waits-001.md) | wait peaks for 30 s or more |
| [MSSQL-REC-LONGQUERY-001](rules/mssql-rec-longquery-001.md) | [PG-REC-LONGQUERY-001](rules/pg-rec-longquery-001.md) | queries running for 1 minute or more |
| [MSSQL-REC-WORKERS-001](rules/mssql-rec-workers-001.md) | [PG-REC-CONN-001](rules/pg-rec-conn-001.md) | worker threads / connections at 90% of the limit |

- **Charts over time**: active requests or sessions, blocked sessions, waits, throughput (batch requests or commits per
  second), page life expectancy, worker threads or connections. Grey bands: no data. Red bands: episodes of a finding.
- **Top queries of the period**, by CPU (SQL Server) or execution time (PostgreSQL).

Every technical number has an **i** button that explains what it means and how to read it. A recording has no A–F
grade: the grade is for `check`.

## What is collected

| Every 15 s | Every 5 min |
|---|---|
| SQL Server: active requests, blocking chains, wait time per type, batch requests, compilations, page life expectancy, worker threads | CPU, duration and reads per query (`sys.dm_exec_query_stats`), top 50 |
| PostgreSQL: active sessions, lock waits, commits, rollbacks, reads, temp bytes and deadlocks per database, connections | time, calls and reads per query (`pg_stat_statements`), top 50 |

Query texts are kept (up to 4 KB) only in the recording folder. Without `pg_stat_statements` the top queries are
**No data**; the rest works. The collection queries are data, like the rules: `rules/<engine>/_collect/` in the
repository.

## Overhead

Target: less than 1% of the server's CPU. Measured with `tools/overhead` (synthetic OLTP load, recording in the middle
of two periods without it), on the test containers:

| | SQL Server 2022 | PostgreSQL 17 |
|---|---|---|
| Recorder share of the server's CPU | **0.003%** | **0.0001%** (execution time, an upper bound) |
| Recorder server time per hour | about 1.9 s | about 0.06 s |
| Load throughput without / with recording | 1953 / 1896 ops/s | 6879 / 6548 ops/s |
| Recording size on these test servers | about 0.04 MB per hour | about 0.04 MB per hour |

Measured on 2026-10-23 on one workstation (20 cores, Docker Desktop), 10 minutes recording between two 5-minute
periods without, both engines at the same time. On that machine the load generator, the databases and Lens share the
CPU, and throughput varied from +7% to −5% between runs with or without recording, so the server time of the
recorder's own queries is the direct measure. A busy server writes more per hour (more sessions and queries per sample).

The `Overhead` workflow repeats the measurement on GitHub Actions (30 minutes recording).
