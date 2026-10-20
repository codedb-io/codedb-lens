WITH f AS (SELECT name, type_desc, size, growth, is_percent_growth FROM sys.master_files WHERE database_id = 2),
     d AS (SELECT COUNT(*) AS n, MIN(size) AS mn, MAX(size) AS mx FROM f WHERE type_desc = 'ROWS'),
     s AS (SELECT CASE WHEN cpu_count > 8 THEN 8 ELSE cpu_count END AS want FROM sys.dm_os_sys_info)
SELECT 'data file count' AS issue, CAST(NULL AS sysname) AS [file], CAST(d.n AS varchar(20)) AS current_value, CAST(s.want AS varchar(20)) AS suggested
FROM d CROSS JOIN s WHERE d.n < s.want
UNION ALL
SELECT 'data files with different sizes', NULL,
       CAST(d.mn * 8 / 1024 AS varchar(20)) + '-' + CAST(d.mx * 8 / 1024 AS varchar(20)) + ' MB', 'same size'
FROM d WHERE d.mn <> d.mx
UNION ALL
SELECT 'growth in percent', f.name, CAST(f.growth AS varchar(20)) + '%', 'fixed MB'
FROM f WHERE f.is_percent_growth = 1;
