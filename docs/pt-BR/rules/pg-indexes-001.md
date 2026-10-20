# PG-INDEXES-001 · Índices não usados, duplicados ou inválidos

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Baixa | Confirmado (inválido, duplicado) · Indício (não usado) | `pg_monitor` + `CONNECT` em cada banco | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

Roda **em cada banco**, sobre `pg_index` e `pg_stat_user_indexes`:

- **Inválido:** `indisvalid = false`, geralmente deixado por um `CREATE INDEX CONCURRENTLY` que falhou.
- **Não usado:** zero scans, não é unique nem chave primária, com pelo menos 10 MB (`min_unused_mb`).
- **Duplicado:** mesmas colunas, classes de operador, expressões e predicado de outro índice da tabela (uma chave primária
  nunca aparece como a duplicada).

## Por que importa

Todo índice é escrito em cada insert e em updates não-HOT, ocupa memória de cache e espaço de backup, e é reconstruído na
manutenção. Índices inválidos são mantidos, mas nunca usados em leituras.

## Próxima ação

```sql
DROP INDEX CONCURRENTLY <índice>;   -- inválido ou duplicado
```

Para os não usados, confirme durante um ciclo completo do negócio e confira `pg_stat_user_indexes` em cada réplica antes de remover.

## Limitações

Os contadores de uso zeram com `pg_stat_reset()`; réplicas têm contadores próprios.
