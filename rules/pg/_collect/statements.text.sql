SELECT /* codeDB Lens */ queryid::text     AS hash,
       left(max(query), 4096) AS text
FROM pg_stat_statements
WHERE queryid IN ({{hashes}})
GROUP BY queryid;
