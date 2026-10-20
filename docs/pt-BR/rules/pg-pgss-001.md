# PG-PGSS-001 · pg_stat_statements indisponível

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Confirmado | `pg_monitor` (`pg_read_all_settings`) | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

Se o `pg_stat_statements` está em `shared_preload_libraries` **e** se a extensão existe no banco em que o Lens conecta.

## Por que importa

É o único registro nativo de quais consultas consomem tempo, I/O e chamadas. Sem ele, achar a consulta a otimizar é chute,
e as regras de consultas do Lens perdem cobertura.

## Próxima ação

```ini
# postgresql.conf (exige restart)
shared_preload_libraries = 'pg_stat_statements'
```

```sql
CREATE EXTENSION pg_stat_statements;
```

No RDS/Aurora use o parameter group; no Azure Flexible Server, o parâmetro `shared_preload_libraries`. Ambos já carregam a
biblioteca por padrão nas versões recentes; só falta o `CREATE EXTENSION`.

## Limitações

A extensão é verificada só no banco em que o Lens conecta.
