-- Cumulative since the last restart; the recorder keeps the difference between runs.
-- Same meaningful waits as MSSQL-WAITS-001: idle and background waits never reach the recording.
SELECT ws.wait_type,
       ws.wait_time_ms        AS wait_ms,
       ws.waiting_tasks_count AS waiting_tasks,
       ws.signal_wait_time_ms AS signal_ms
FROM sys.dm_os_wait_stats ws
WHERE ws.wait_time_ms > 0
  AND (ws.wait_type LIKE 'PAGEIOLATCH[_]%'
       OR ws.wait_type LIKE 'LCK[_]M[_]%'
       OR ws.wait_type LIKE 'PAGELATCH[_]%'
       OR ws.wait_type IN ('WRITELOG', 'IO_COMPLETION', 'SOS_SCHEDULER_YIELD', 'CXPACKET', 'CXCONSUMER',
                           'RESOURCE_SEMAPHORE', 'THREADPOOL', 'ASYNC_NETWORK_IO', 'HADR_SYNC_COMMIT'));
