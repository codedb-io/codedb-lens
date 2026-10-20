WITH q AS (
    SELECT s.dbid, s.calls, s.total_exec_time, s.mean_exec_time, s.rows,
           s.shared_blks_read, s.shared_blks_hit, s.query
    FROM pg_stat_statements s
    WHERE s.toplevel
),
t AS (SELECT sum(total_exec_time) AS total, max(total_exec_time) AS top1 FROM q),
top5 AS (SELECT * FROM q ORDER BY total_exec_time DESC LIMIT 5)
SELECT round((100 * top5.total_exec_time / t.total)::numeric, 1) AS time_pct,
       top5.calls,
       round(top5.mean_exec_time::numeric, 1) AS mean_ms,
       round((top5.total_exec_time / 1000)::numeric, 0) AS total_s,
       top5.rows / nullif(top5.calls, 0) AS rows_per_call,
       round((100.0 * top5.shared_blks_hit / nullif(top5.shared_blks_hit + top5.shared_blks_read, 0))::numeric, 1) AS cache_hit_pct,
       d.datname AS database,
       left(regexp_replace(top5.query, '\s+', ' ', 'g'), 300) AS query
FROM top5
CROSS JOIN t
LEFT JOIN pg_database d ON d.oid = top5.dbid
WHERE t.total / 1000 >= {{min_total_s}}
  AND 100 * t.top1 / t.total >= {{dominant_pct}}
ORDER BY top5.total_exec_time DESC;
