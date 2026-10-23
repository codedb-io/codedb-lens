-- pg_stat_statements(false) skips reading the query texts file; texts come from statements.text.sql, top 50 only.
-- Without the extension the query fails and the recording marks the collector as "no data".
SELECT queryid::text                   AS query_hash,
       sum(calls)                      AS calls,
       round(sum(total_exec_time)::numeric, 1) AS total_ms,
       sum(rows)                       AS rows,
       sum(shared_blks_read)           AS blks_read,
       sum(shared_blks_hit)            AS blks_hit,
       sum(temp_blks_written)          AS temp_blks
FROM pg_stat_statements(false)
WHERE queryid IS NOT NULL
GROUP BY queryid;
