# PG-TOPQUERY-001 · Uma única consulta domina o tempo de execução

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Indício | `pg_monitor` (`pg_read_all_stats`) + `pg_stat_statements` | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

Comandos de nível superior no `pg_stat_statements`. Aparece quando a consulta mais cara leva pelo menos 40% do tempo total
de execução (`dominant_pct`) e o total é de pelo menos 60 s (`min_total_s`). A evidência lista as 5 maiores com
participação, chamadas, tempo médio, linhas por chamada e taxa de acerto no cache.

Sem a extensão, a regra fica *Sem dados* e aponta para o `PG-PGSS-001`.

## Por que importa

Quando uma consulta domina, corrigi-la é a mudança de maior retorno.

## Próxima ação

```sql
EXPLAIN (ANALYZE, BUFFERS) <consulta com parâmetros reais>;
```

Procure scans sequenciais em tabelas grandes, estimativas de linhas muito longe da realidade, e sorts ou hashes indo para
o disco. Veja também se ela roda mais do que a aplicação precisa (poucas *linhas por chamada* com *chamadas* enormes
sugere o padrão N+1).

## Limitações

Os totais acumulam desde o último `pg_stat_statements_reset()` ou restart, então não dá para saber desde quando a consulta
domina. Sem `pg_read_all_stats`, os textos das consultas de outros usuários aparecem como `<insufficient privilege>`.
