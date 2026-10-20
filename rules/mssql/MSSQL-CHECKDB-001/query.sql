SELECT d.name AS [database],
       NULLIF(CAST(DATABASEPROPERTYEX(d.name, 'LastGoodCheckDbTime') AS datetime), '19000101') AS last_good_checkdb,
       DATEDIFF(day, NULLIF(CAST(DATABASEPROPERTYEX(d.name, 'LastGoodCheckDbTime') AS datetime), '19000101'), GETDATE()) AS days_ago,
       d.create_date
FROM sys.databases d
WHERE d.database_id <> 2                -- tempdb
  AND d.state_desc = 'ONLINE'
  AND d.source_database_id IS NULL      -- snapshots
  AND d.create_date < DATEADD(day, -{{max_days}}, GETDATE())
  AND ISNULL(CAST(DATABASEPROPERTYEX(d.name, 'LastGoodCheckDbTime') AS datetime), '19000101') < DATEADD(day, -{{max_days}}, GETDATE())
ORDER BY d.name;
