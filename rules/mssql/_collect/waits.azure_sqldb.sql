-- Azure SQL Database: waits of this database only (sys.dm_db_wait_stats).
SELECT ws.wait_type,
       ws.wait_time_ms        AS wait_ms,
       ws.waiting_tasks_count AS waiting_tasks,
       ws.signal_wait_time_ms AS signal_ms
FROM sys.dm_db_wait_stats ws
WHERE ws.wait_time_ms > 0
  AND (ws.wait_type LIKE 'PAGEIOLATCH[_]%'
       OR ws.wait_type LIKE 'LCK[_]M[_]%'
       OR ws.wait_type LIKE 'PAGELATCH[_]%'
       OR ws.wait_type IN ('WRITELOG', 'IO_COMPLETION', 'SOS_SCHEDULER_YIELD', 'CXPACKET', 'CXCONSUMER',
                           'RESOURCE_SEMAPHORE', 'THREADPOOL', 'ASYNC_NETWORK_IO', 'HADR_SYNC_COMMIT'));
