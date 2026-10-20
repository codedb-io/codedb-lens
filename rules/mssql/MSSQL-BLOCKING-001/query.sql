SELECT r.session_id                    AS blocked_session,
       r.blocking_session_id           AS blocked_by,
       DB_NAME(r.database_id)          AS [database],
       CAST(r.wait_time / 1000.0 AS decimal(12, 1)) AS waiting_s,
       r.wait_type,
       bs.status                       AS blocker_status,
       bs.login_name                   AS blocker_login,
       bs.host_name                    AS blocker_host,
       bs.program_name                 AS blocker_program,
       bs.open_transaction_count       AS blocker_open_trans,
       LEFT(REPLACE(REPLACE(t.text, CHAR(13), ' '), CHAR(10), ' '), 200) AS blocked_query
FROM sys.dm_exec_requests r
JOIN sys.dm_exec_sessions bs ON bs.session_id = r.blocking_session_id
OUTER APPLY sys.dm_exec_sql_text(r.sql_handle) t
WHERE r.blocking_session_id <> 0
  AND r.session_id <> @@SPID
  AND r.wait_time >= {{min_wait_ms}}
ORDER BY r.wait_time DESC;
