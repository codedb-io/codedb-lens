-- Cumulative per query since its plan was cached; the recorder keeps the difference between runs.
-- Only queries run in the last 6 min (the slow cycle is 5): the others have no delta, and skipping them keeps the
-- result small on servers with a large plan cache. Texts come from querystats.text.sql, for the top 50 only.
SELECT /* codeDB Lens */ CONVERT(varchar(18), qs.query_hash, 1) AS query_hash,
       SUM(qs.execution_count)                AS executions,
       SUM(qs.total_worker_time)              AS cpu_us,
       SUM(qs.total_elapsed_time)             AS elapsed_us,
       SUM(qs.total_logical_reads)            AS logical_reads,
       SUM(qs.total_physical_reads)           AS physical_reads
FROM sys.dm_exec_query_stats qs
WHERE qs.last_execution_time >= DATEADD(second, -360, SYSDATETIME())
GROUP BY qs.query_hash;
