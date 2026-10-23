# MSSQL-REC-LONGQUERY-001 · Queries running for 1 minute or more

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Confirmed | `VIEW SERVER STATE` | Recordings (`codedb-lens record`): SQL Server 2017+, Azure SQL MI, RDS |

## What it checks

The `requests` collector (`sys.dm_exec_requests`, every 15 s, `WAITFOR` left out). One episode per session and query while it has been running at least 60 s (`min_s`).

## Why it matters

Requests ran for at least a minute. Long queries hold locks and memory and compete with everything else; in an online system they are usually what users feel as slowness.

## Next action

Separate what is expected (backups, index maintenance, reports at night) from what is not. For the rest, look at the plan of the query (its text is in the report) and at missing indexes and wait types in the same period.

## How to verify

Record again during the same hours after the change. The query must not appear, or its peak duration must be clearly lower.

## Limitations

Samples are taken every 15 seconds, so the real duration may be up to 15 seconds longer than the peak shown. Maintenance jobs show up here too.
