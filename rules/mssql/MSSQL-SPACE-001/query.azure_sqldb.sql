-- Azure SQL Database: no volumes to see. The limit is the database's max size: once it is reached, inserts
-- and updates fail. Hyperscale reports no max size (-1) and is skipped. Volume params are reused for the max size.
WITH used AS (
    SELECT SUM(CAST(FILEPROPERTY(name, 'SpaceUsed') AS bigint)) * 8 / 1024 AS used_mb
    FROM sys.database_files
    WHERE type_desc = 'ROWS'
),
lim AS (SELECT CAST(DATABASEPROPERTYEX(DB_NAME(), 'MaxSizeInBytes') AS bigint) / 1048576 AS max_mb)
SELECT 'database'                                                          AS kind,
       DB_NAME()                                                           AS name,
       CAST((lim.max_mb - used.used_mb) * 100.0 / lim.max_mb AS decimal(5, 1)) AS free_pct,
       lim.max_mb - used.used_mb                                           AS free_mb,
       CAST(NULL AS decimal(5, 1))                                         AS log_used_pct,
       CAST(NULL AS nvarchar(60))                                          AS log_reuse_wait,
       CASE WHEN (lim.max_mb - used.used_mb) * 100.0 / lim.max_mb < {{volume_confirmed_pct}} THEN 'confirmed' ELSE 'hint' END AS _confidence
FROM used CROSS JOIN lim
WHERE lim.max_mb > 0
  AND (lim.max_mb - used.used_mb) * 100.0 / lim.max_mb < {{volume_hint_pct}}
UNION ALL
SELECT 'log', d.name, NULL, NULL,
       CAST(ls.active_log_size_mb * 100.0 / NULLIF(ls.total_log_size_mb, 0) AS decimal(5, 1)),
       d.log_reuse_wait_desc,
       CASE WHEN ls.active_log_size_mb * 100.0 / NULLIF(ls.total_log_size_mb, 0) >= {{log_confirmed_pct}} THEN 'confirmed' ELSE 'hint' END
FROM sys.databases d
CROSS APPLY sys.dm_db_log_stats(d.database_id) ls
WHERE d.database_id = DB_ID()
  AND d.log_reuse_wait_desc NOT IN ('NOTHING', 'CHECKPOINT')
  AND ls.active_log_size_mb * 100.0 / NULLIF(ls.total_log_size_mb, 0) >= {{log_hint_pct}};
