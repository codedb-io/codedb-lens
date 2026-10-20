WITH h AS (
    SELECT jh.job_id, jh.run_status, jh.message,
           CONVERT(datetime, CONVERT(char(8), jh.run_date))
             + CONVERT(datetime, STUFF(STUFF(RIGHT('000000' + CAST(jh.run_time AS varchar(6)), 6), 5, 0, ':'), 3, 0, ':')) AS run_at,
           ROW_NUMBER() OVER (PARTITION BY jh.job_id ORDER BY jh.instance_id DESC) AS rn
    FROM msdb.dbo.sysjobhistory jh
    WHERE jh.step_id = 0                -- job outcome
)
SELECT j.name AS job,
       h.run_at AS last_run,
       CASE h.run_status WHEN 0 THEN 'failed' WHEN 3 THEN 'canceled' END AS outcome,
       (SELECT COUNT(*) FROM h AS x WHERE x.job_id = j.job_id AND x.run_status = 0
          AND x.run_at >= DATEADD(day, -{{days}}, GETDATE())) AS failures_in_window,
       LEFT(h.message, 300) AS message
FROM msdb.dbo.sysjobs j
JOIN h ON h.job_id = j.job_id AND h.rn = 1
WHERE j.enabled = 1
  AND h.run_status IN (0, 3)
  AND h.run_at >= DATEADD(day, -{{days}}, GETDATE())
ORDER BY h.run_at DESC;
