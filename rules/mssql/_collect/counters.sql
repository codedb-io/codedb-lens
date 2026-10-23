-- One row. Batch requests and compilations are cumulative counters despite the "/sec" name.
SELECT /* codeDB Lens */ MAX(CASE WHEN RTRIM(counter_name) = 'Batch Requests/sec' THEN cntr_value END)   AS batch_requests,
       MAX(CASE WHEN RTRIM(counter_name) = 'SQL Compilations/sec' THEN cntr_value END) AS compilations,
       MIN(CASE WHEN RTRIM(counter_name) = 'Page life expectancy'
                 AND object_name LIKE '%Buffer Manager%' THEN cntr_value END)          AS ple_s,
       MAX(CASE WHEN RTRIM(counter_name) = 'User Connections' THEN cntr_value END)     AS user_connections
FROM sys.dm_os_performance_counters
WHERE RTRIM(counter_name) IN ('Batch Requests/sec', 'SQL Compilations/sec', 'Page life expectancy', 'User Connections');
