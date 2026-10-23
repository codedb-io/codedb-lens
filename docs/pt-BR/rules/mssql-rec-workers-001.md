# MSSQL-REC-WORKERS-001 · Threads de trabalho perto do limite

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Confirmado | `VIEW SERVER STATE` | Gravações (`codedb-lens record`): SQL Server 2017+, Azure SQL MI, RDS |

## O que verifica

O coletor `workers`: threads de trabalho em `sys.dm_os_schedulers` como porcentagem de `max_workers_count`. Um episódio são duas ou mais amostras seguidas com 90% ou mais (`max_pct`).

## Por que importa

O SQL Server usou 90% ou mais das suas threads de trabalho por pelo menos 30 segundos. Quando acabam, novas requisições esperam (THREADPOOL) e até novas conexões podem falhar, mesmo com a CPU parecendo ociosa.

## Próxima ação

A causa mais comum é bloqueio (muitas sessões presas atrás de uma) ou paralelismo excessivo. Veja primeiro os achados de bloqueio e de espera do mesmo período; não aumente o max worker threads antes de achar a causa.

## Como validar

Grave de novo no mesmo horário depois da correção. As threads devem ficar bem abaixo de max_workers.

## Limitações

Conta as threads criadas pelo SQL Server, que continuam existindo por um tempo depois de um pico.
