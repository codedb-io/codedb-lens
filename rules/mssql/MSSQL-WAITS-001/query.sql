-- Only waits with a clear meaning are counted; idle and background waits never reach the report.
WITH meaning (pattern, cat_en, cat_pt) AS (
    SELECT * FROM (VALUES
        ('PAGEIOLATCH[_]%',         'Disk: reading data pages',                     'Disco: leitura de páginas de dados'),
        ('WRITELOG',                'Disk: transaction log writes',                 'Disco: escrita no log de transações'),
        ('IO[_]COMPLETION',         'Disk: other I/O',                              'Disco: outros I/O'),
        ('LCK[_]M[_]%',             'Blocking: waiting for locks',                  'Bloqueio: esperando locks'),
        ('PAGELATCH[_]%',           'Page contention in memory (often TempDB)',     'Disputa de páginas em memória (geralmente TempDB)'),
        ('SOS[_]SCHEDULER[_]YIELD', 'CPU pressure',                                 'Pressão de CPU'),
        ('CXPACKET',                'Parallelism',                                  'Paralelismo'),
        ('CXCONSUMER',              'Parallelism',                                  'Paralelismo'),
        ('RESOURCE[_]SEMAPHORE',    'Queries waiting for memory grants',            'Consultas esperando memória'),
        ('THREADPOOL',              'Out of worker threads',                        'Falta de threads de trabalho'),
        ('ASYNC[_]NETWORK[_]IO',    'Application not reading results fast enough',  'Aplicação lendo resultados devagar'),
        ('HADR[_]SYNC[_]COMMIT',    'Synchronous Availability Group commit',        'Commit síncrono do Availability Group')
    ) v (pattern, cat_en, cat_pt)
),
c AS (
    SELECT m.cat_en, MAX(m.cat_pt) AS cat_pt, SUM(ws.wait_time_ms) AS ms, SUM(ws.waiting_tasks_count) AS tasks,
           MAX(ws.wait_type) AS example
    FROM sys.dm_os_wait_stats ws
    JOIN meaning m ON ws.wait_type LIKE m.pattern
    WHERE ws.wait_time_ms > 0
    GROUP BY m.cat_en
),
t AS (SELECT SUM(ms) AS total FROM c),
up AS (SELECT DATEDIFF(second, sqlserver_start_time, SYSDATETIME()) / 3600.0 AS hours FROM sys.dm_os_sys_info)
SELECT c.cat_en AS [meaning@en],
       c.cat_pt AS [meaning@pt-BR],
       c.example AS wait_type,
       CAST(c.ms * 100.0 / t.total AS decimal(5, 1)) AS share_pct,
       CAST(c.ms / 1000.0 / NULLIF(up.hours, 0) AS decimal(14, 1)) AS wait_s_per_hour,
       CAST(c.ms * 1.0 / NULLIF(c.tasks, 0) AS decimal(14, 1)) AS avg_wait_ms,
       CASE WHEN c.cat_en IN ('Out of worker threads', 'Queries waiting for memory grants') THEN 'confirmed' ELSE 'hint' END AS _confidence
FROM c CROSS JOIN t CROSS JOIN up
WHERE c.ms * 100.0 / t.total >= {{min_share_pct}}
  AND c.ms / 1000.0 / NULLIF(up.hours, 0) >= {{min_wait_s_per_hour}}
ORDER BY c.ms DESC;
