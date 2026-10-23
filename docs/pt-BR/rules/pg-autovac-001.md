# PG-AUTOVAC-001 · Muitas linhas mortas (autovacuum ficando para trás)

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Indício · Confirmado a 50% de linhas mortas | `pg_monitor` + `CONNECT` em cada banco | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

Roda **em cada banco**. Tabelas em `pg_stat_user_tables` com pelo menos 10.000 linhas mortas (`min_dead`) que somam pelo
menos 20% da tabela (`dead_pct`), com o horário do último vacuum e analyze. Top 20.

## Por que importa

Linhas mortas são versões deixadas por updates e deletes. Até o vacuum removê-las, os scans as leem mesmo assim e o espaço
não pode ser reaproveitado, então tabelas e índices crescem (bloat).

## Próxima ação

1. Veja o que trava o vacuum: transações antigas (`PG-LONGTX-001`), slots abandonados (`PG-SLOTS-001`), prepared transactions antigas.
2. Em tabelas muito alteradas, faça o autovacuum disparar antes:

```sql
ALTER TABLE <tabela> SET (autovacuum_vacuum_scale_factor = 0.02, autovacuum_vacuum_threshold = 1000);
```

3. Aumente o `autovacuum_vacuum_cost_limit` se o autovacuum roda mas nunca dá conta.

## Como validar

Confira a tabela: `SELECT n_dead_tup, last_autovacuum, last_vacuum FROM pg_stat_user_tables WHERE relname = 'tabela'`. Depois do vacuum, `n_dead_tup` deve cair bastante e `last_autovacuum` ou `last_vacuum` deve ser recente. Depois acompanhe por um dia: se as linhas mortas voltarem a subir acima do limite, o autovacuum ainda não dá conta e a causa (transações antigas, slots, parâmetros) continua.

## Limitações

Os contadores zeram com `pg_stat_reset()` e após um crash. Uma tabela com `autovacuum_enabled = false` aparece como qualquer outra.
