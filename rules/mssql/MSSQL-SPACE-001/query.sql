WITH vol AS (
    SELECT DISTINCT s.volume_mount_point, s.total_bytes, s.available_bytes
    FROM sys.master_files f
    CROSS APPLY sys.dm_os_volume_stats(f.database_id, f.file_id) s
)
SELECT 'volume'                                                AS kind,
       v.volume_mount_point                                    AS name,
       CAST(v.available_bytes * 100.0 / v.total_bytes AS decimal(5, 1)) AS free_pct,
       CAST(v.available_bytes / 1048576 AS bigint)             AS free_mb,
       CAST(NULL AS decimal(5, 1))                             AS log_used_pct,
       CAST(NULL AS nvarchar(60))                              AS log_reuse_wait,
       CASE WHEN v.available_bytes * 100.0 / v.total_bytes < {{volume_confirmed_pct}} THEN 'confirmed' ELSE 'hint' END AS _confidence
FROM vol v
WHERE v.available_bytes * 100.0 / v.total_bytes < {{volume_hint_pct}}
UNION ALL
SELECT 'log', d.name, NULL, NULL,
       CAST(ls.active_log_size_mb * 100.0 / NULLIF(ls.total_log_size_mb, 0) AS decimal(5, 1)),
       d.log_reuse_wait_desc,
       CASE WHEN ls.active_log_size_mb * 100.0 / NULLIF(ls.total_log_size_mb, 0) >= {{log_confirmed_pct}} THEN 'confirmed' ELSE 'hint' END
FROM sys.databases d
CROSS APPLY sys.dm_db_log_stats(d.database_id) ls  -- per-database counters are unreliable on 2025
WHERE d.state_desc = 'ONLINE'
  AND d.log_reuse_wait_desc NOT IN ('NOTHING', 'CHECKPOINT')
  AND ls.active_log_size_mb * 100.0 / NULLIF(ls.total_log_size_mb, 0) >= {{log_hint_pct}};
