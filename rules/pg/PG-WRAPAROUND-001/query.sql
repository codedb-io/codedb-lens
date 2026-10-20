SELECT d.datname AS database,
       age(d.datfrozenxid) AS xid_age,
       current_setting('autovacuum_freeze_max_age')::bigint AS autovacuum_freeze_max_age,
       round(100.0 * age(d.datfrozenxid) / 2147483647, 1) AS pct_to_wraparound,
       CASE WHEN age(d.datfrozenxid) >= {{confirmed_xid_age}} THEN 'confirmed' ELSE 'hint' END AS _confidence
FROM pg_database d
WHERE d.datallowconn
  AND age(d.datfrozenxid) >= {{hint_ratio}} * current_setting('autovacuum_freeze_max_age')::bigint
ORDER BY age(d.datfrozenxid) DESC;
