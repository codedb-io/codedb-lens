# MSSQL-TOPQUERY-001 · Uma única consulta domina a CPU

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Indício | `VIEW SERVER STATE` | SQL Server 2017+, Azure SQL MI, RDS, Azure SQL Database |

## O que verifica

`sys.dm_exec_query_stats` agrupado por `query_hash`. Aparece quando a consulta mais cara usa pelo menos 40% da CPU
registrada no plan cache (`dominant_pct`) e o cache tem pelo menos 60 s de CPU (`min_total_cpu_s`). A evidência lista as
5 maiores com participação, execuções, CPU média, duração e leituras.

No Azure SQL Database o cache de planos cobre só o banco conectado.

## Por que importa

Otimizar um banco quase nunca é otimizar tudo. Quando uma consulta domina, corrigi-la é a mudança de maior retorno.

## Próxima ação

Pegue o plano da consulta do topo (`sys.dm_exec_query_plan`, ou o Query Store se estiver ligado) e procure scans, índices
faltando, conversões implícitas e estimativas ruins. Verifique também se ela roda mais vezes do que a aplicação precisa.

## Limitações

O plan cache é esvaziado no restart e sob pressão de memória; consultas com `OPTION (RECOMPILE)` podem faltar. Sem
histórico, não dá para saber desde quando a consulta domina.
