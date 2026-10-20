# MSSQL-MISSINGIDX-001 · Índices ausentes de alto impacto

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Indício (sempre) | `VIEW SERVER STATE` | SQL Server 2017+, Azure SQL MI, RDS |

## O que verifica

As DMVs de índices ausentes (`sys.dm_db_missing_index_*`) dos bancos de usuário. Uma sugestão aparece quando o otimizador
estima pelo menos 50% de melhora (`min_impact_pct`) e o benefício ponderado
`avg_total_user_cost × avg_user_impact × (seeks + scans)` é de pelo menos 100.000 (`min_improvement`). Top 20.

## Por que importa

Cada linha é uma consulta que o otimizador compilou desejando um índice que não existe. Essas consultas leem mais páginas
e seguram locks por mais tempo.

## Próxima ação

Não crie as sugestões como vêm. Para cada tabela: compare com os índices existentes, junte sugestões sobrepostas numa só,
coloque as colunas de igualdade primeiro e teste numa cópia antes de produção.

## Limitações

- Sempre indício: a DMV não olha índices parecidos que já existem e costuma sugerir índices sobrepostos.
- É zerada no restart e quando qualquer índice é criado ou removido na tabela.
- Consultas com plano trivial nunca geram sugestões.
