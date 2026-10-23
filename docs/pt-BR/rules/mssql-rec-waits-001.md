# MSSQL-REC-WAITS-001 · Picos de espera durante a gravação

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Indício | `VIEW SERVER STATE` | Gravações (`codedb-lens record`): SQL Server 2017+, Azure SQL MI, RDS |

## O que verifica

O coletor `waits`: tempo de espera por tipo em `sys.dm_os_wait_stats`, como diferença entre amostras, dividido pelos segundos entre elas. Um episódio são duas ou mais amostras seguidas com 2000 ms de espera por segundo ou mais (`min_wait_ms_per_s`).

## Por que importa

Por pelo menos 30 segundos, as queries passaram muito tempo esperando um mesmo recurso (disco, locks, CPU, memória, log). Cada tipo de espera aponta um gargalo diferente.

## Próxima ação

Relacione o pico ao tipo de espera: PAGEIOLATCH = leitura de dados em disco (índices ou memória faltando), WRITELOG = disco do log, LCK_M = bloqueio, SOS_SCHEDULER_YIELD = CPU, RESOURCE_SEMAPHORE = memória para queries, CXPACKET/CXCONSUMER = paralelismo. Depois veja as top queries do mesmo período.

## Como validar

Grave de novo no mesmo horário depois da mudança. Os picos desse tipo de espera devem ficar menores ou mais curtos.

## Limitações

Um pico de espera é sintoma, não causa, por isso é um indício. Só esperas com significado claro são gravadas; esperas ociosas e de segundo plano ficam de fora.
