WITH c AS (SELECT name, CAST(value_in_use AS bigint) AS v FROM sys.configurations),
     s AS (SELECT cpu_count, physical_memory_kb / 1024 AS ram_mb FROM sys.dm_os_sys_info)
SELECT 'max server memory (MB)' AS setting, CAST(c.v AS varchar(20)) AS current_value,
       CAST(CAST(s.ram_mb * 0.8 AS bigint) AS varchar(20)) AS suggested, CAST(s.ram_mb AS varchar(20)) + ' MB RAM' AS host
FROM c CROSS JOIN s WHERE c.name = 'max server memory (MB)' AND c.v >= 2147483647
UNION ALL
SELECT 'max degree of parallelism', CAST(c.v AS varchar(20)), '8', CAST(s.cpu_count AS varchar(20)) + ' CPUs'
FROM c CROSS JOIN s WHERE c.name = 'max degree of parallelism' AND c.v = 0 AND s.cpu_count > {{maxdop_cpus}}
UNION ALL
SELECT 'cost threshold for parallelism', CAST(c.v AS varchar(20)), '50', CAST(s.cpu_count AS varchar(20)) + ' CPUs'
FROM c CROSS JOIN s WHERE c.name = 'cost threshold for parallelism' AND c.v = 5
UNION ALL
SELECT 'optimize for ad hoc workloads', CAST(c.v AS varchar(20)), '1', NULL
FROM c WHERE c.name = 'optimize for ad hoc workloads' AND c.v = 0;
