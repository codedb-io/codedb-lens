SELECT d.name AS [database],
       d.is_auto_shrink_on,
       d.is_auto_close_on,
       d.page_verify_option_desc AS page_verify
FROM sys.databases d
WHERE d.database_id > 4                 -- user databases only
  AND d.state_desc = 'ONLINE'
  AND d.source_database_id IS NULL      -- snapshots
  AND (d.is_auto_shrink_on = 1 OR d.is_auto_close_on = 1 OR d.page_verify_option_desc <> 'CHECKSUM')
ORDER BY d.name;
