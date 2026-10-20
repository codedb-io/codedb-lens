WITH q AS (
    SELECT qs.query_hash,
           SUM(qs.total_worker_time)   AS cpu_us,
           SUM(qs.execution_count)     AS executions,
           SUM(qs.total_logical_reads) AS reads,
           SUM(qs.total_elapsed_time)  AS elapsed_us,
           MAX(qs.sql_handle)          AS sql_handle,
           MAX(qs.statement_start_offset) AS so,
           MAX(qs.statement_end_offset)   AS eo
    FROM sys.dm_exec_query_stats qs
    GROUP BY qs.query_hash
),
t AS (SELECT SUM(cpu_us) AS total, MAX(cpu_us) AS top1 FROM q),
top5 AS (SELECT TOP (5) * FROM q ORDER BY cpu_us DESC)
SELECT CAST(top5.cpu_us * 100.0 / t.total AS decimal(5, 1)) AS cpu_pct,
       top5.executions,
       CAST(top5.cpu_us / 1000.0 / NULLIF(top5.executions, 0) AS decimal(14, 1))     AS avg_cpu_ms,
       CAST(top5.elapsed_us / 1000.0 / NULLIF(top5.executions, 0) AS decimal(14, 1)) AS avg_duration_ms,
       top5.reads / NULLIF(top5.executions, 0) AS avg_reads,
       DB_NAME(st.dbid) AS [database],
       LEFT(REPLACE(REPLACE(SUBSTRING(st.text, top5.so / 2 + 1,
            (CASE top5.eo WHEN -1 THEN DATALENGTH(st.text) ELSE top5.eo END - top5.so) / 2 + 1),
            CHAR(13), ' '), CHAR(10), ' '), 300) AS query
FROM top5 CROSS JOIN t
OUTER APPLY sys.dm_exec_sql_text(top5.sql_handle) st
WHERE t.total / 1000000.0 >= {{min_total_cpu_s}}
  AND t.top1 * 100.0 / t.total >= {{dominant_pct}}
ORDER BY top5.cpu_us DESC;
