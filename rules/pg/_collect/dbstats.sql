-- Cumulative since the last stats reset; the recorder keeps the difference between runs.
SELECT /* codeDB Lens */ datname       AS database,
       xact_commit   AS commits,
       xact_rollback AS rollbacks,
       blks_read,
       blks_hit,
       temp_bytes,
       deadlocks
FROM pg_stat_database
WHERE datname IS NOT NULL
  AND datname NOT IN ('template0', 'template1');
