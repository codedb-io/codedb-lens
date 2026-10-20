# PG-BLOAT-001 · Inchaço (bloat) estimado de tabelas

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Indício (sempre estimativa) | `pg_monitor` + `CONNECT` em cada banco + `SELECT` nas tabelas (ou `pg_read_all_data`) | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

Roda **em cada banco**, para tabelas com pelo menos 100 MB (`min_size_mb`). Estima quantas páginas a tabela *deveria*
precisar, pelo número de linhas e pela largura média da linha (`pg_stats`, respeitando o `fillfactor`), e compara com as
páginas que ela usa. Aparece a partir de 40% de desperdício estimado (`bloat_pct`).

## Estatísticas e permissões

O `pg_stats` só mostra colunas que o login pode ler com `SELECT`. O Lens não chuta:

- Tabelas nunca analisadas são ignoradas.
- Se tabelas grandes têm estatísticas que esse login não enxerga, a regra fica **Sem dados** com a contagem, em vez de um
  falso OK. Conceda `SELECT` nessas tabelas, ou `pg_read_all_data` (PostgreSQL 14+), se quiser esta regra.

## Por que importa

Uma tabela inchada custa mais disco, mais cache e mais I/O para as mesmas linhas. O `VACUUM` comum torna o espaço
reutilizável dentro da tabela, mas não o devolve.

## Próxima ação

Confirme antes com `pgstattuple_approx('<tabela>')`. Recupere o espaço sem parar a tabela com `pg_repack` ou `pg_squeeze`;
o `VACUUM FULL` segura um lock exclusivo durante toda a execução. Depois corrija a causa (`PG-AUTOVAC-001`, `PG-LONGTX-001`).

## Limitações

Só a tabela (índices não são estimados). A estimativa assume estatísticas atuais; rode `ANALYZE` se estiverem antigas.
