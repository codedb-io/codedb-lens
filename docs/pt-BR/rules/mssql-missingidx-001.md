# MSSQL-MISSINGIDX-001 · Índices ausentes de alto impacto

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Indício (sempre) | `VIEW SERVER STATE` | SQL Server 2017+, Azure SQL MI, RDS, Azure SQL Database |

## O que verifica

As DMVs de índices ausentes (`sys.dm_db_missing_index_*`) dos bancos de usuário. Uma sugestão aparece quando o otimizador
estima pelo menos 50% de melhora (`min_impact_pct`) e o benefício ponderado
`avg_total_user_cost × avg_user_impact × (seeks + scans)` é de pelo menos 100.000 (`min_improvement`). Top 20.

No Azure SQL Database só o banco conectado é verificado.

## Por que importa

Cada linha é uma consulta que o otimizador compilou desejando um índice que não existe. Essas consultas leem mais páginas
e seguram locks por mais tempo.

## Próxima ação

Não crie as sugestões como vêm. Para cada tabela: compare com os índices existentes, junte sugestões sobrepostas numa só,
coloque as colunas de igualdade primeiro e teste numa cópia antes de produção.

## Como validar

Depois de criar o índice, rode a consulta que o motivou e compare leituras lógicas e duração com `SET STATISTICS IO, TIME ON` (ou o plano real: seek no índice novo em vez de scan). Mais tarde, confirme que ele é usado: `SELECT i.name, s.user_seeks, s.user_scans FROM sys.indexes i JOIN sys.dm_db_index_usage_stats s ON s.object_id = i.object_id AND s.index_id = i.index_id AND s.database_id = DB_ID() WHERE i.name = N'nome_do_indice'`. A sugestão sai da DMV depois do próximo restart ou de mudança no índice.

## Limitações

- Sempre indício: a DMV não olha índices parecidos que já existem e costuma sugerir índices sobrepostos.
- É zerada no restart e quando qualquer índice é criado ou removido na tabela.
- Consultas com plano trivial nunca geram sugestões.
