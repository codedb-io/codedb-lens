-- Worker threads are what runs out on SQL Server (THREADPOOL), not connections.
SELECT SUM(s.current_workers_count) AS workers,
       SUM(s.active_workers_count)  AS active_workers,
       SUM(s.work_queue_count)      AS queued_tasks,
       SUM(s.runnable_tasks_count)  AS runnable_tasks,
       MAX(i.max_workers_count)     AS max_workers
FROM sys.dm_os_schedulers s
CROSS JOIN sys.dm_os_sys_info i
WHERE s.status = 'VISIBLE ONLINE';
