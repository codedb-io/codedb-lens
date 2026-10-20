WITH b AS (
    SELECT database_name,
           MAX(CASE WHEN type = 'D' THEN backup_finish_date END) AS last_full,
           MAX(CASE WHEN type = 'I' THEN backup_finish_date END) AS last_diff,
           MAX(CASE WHEN type = 'L' THEN backup_finish_date END) AS last_log
    FROM msdb.dbo.backupset
    WHERE is_copy_only = 0
    GROUP BY database_name
)
SELECT d.name                AS [database],
       d.recovery_model_desc AS recovery_model,
       b.last_full,
       b.last_diff,
       b.last_log,
       CASE
           WHEN b.last_full IS NULL THEN 'no full backup'
           WHEN ISNULL(b.last_diff, b.last_full) < DATEADD(hour, -{{full_max_hours}}, GETDATE())
                AND b.last_full < DATEADD(hour, -{{full_max_hours}}, GETDATE()) THEN 'full/diff too old'
           ELSE 'log backup too old'
       END AS problem
FROM sys.databases d
LEFT JOIN b ON b.database_name = d.name
WHERE d.database_id <> 2               -- tempdb
  AND d.state_desc = 'ONLINE'
  AND d.source_database_id IS NULL     -- snapshots
  AND (
        b.last_full IS NULL
     OR (ISNULL(b.last_diff, b.last_full) < DATEADD(hour, -{{full_max_hours}}, GETDATE())
         AND b.last_full < DATEADD(hour, -{{full_max_hours}}, GETDATE()))
     OR (d.recovery_model_desc <> 'SIMPLE' AND d.name <> 'model'
         AND ISNULL(b.last_log, '19000101') < DATEADD(hour, -{{log_max_hours}}, GETDATE()))
  )
ORDER BY d.name;
