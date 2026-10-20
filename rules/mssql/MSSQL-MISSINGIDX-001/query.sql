SELECT TOP (20)
       DB_NAME(d.database_id) AS [database],
       d.statement AS [table],
       d.equality_columns,
       d.inequality_columns,
       d.included_columns,
       s.user_seeks + s.user_scans AS uses,
       CAST(s.avg_user_impact AS decimal(5, 1)) AS est_impact_pct,
       CAST(s.avg_total_user_cost * s.avg_user_impact * (s.user_seeks + s.user_scans) AS decimal(18, 0)) AS improvement,
       s.last_user_seek
FROM sys.dm_db_missing_index_details d
JOIN sys.dm_db_missing_index_groups g ON g.index_handle = d.index_handle
JOIN sys.dm_db_missing_index_group_stats s ON s.group_handle = g.index_group_handle
WHERE d.database_id > 4
  AND s.avg_user_impact >= {{min_impact_pct}}
  AND s.avg_total_user_cost * s.avg_user_impact * (s.user_seeks + s.user_scans) >= {{min_improvement}}
ORDER BY improvement DESC;
