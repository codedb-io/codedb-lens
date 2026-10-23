# MSSQL-REC-BLOCKING-001 · Bloqueios de 30 s ou mais durante a gravação

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Confirmado | `VIEW SERVER STATE` | Gravações (`codedb-lens record`): SQL Server 2017+, Azure SQL MI, RDS |

## O que verifica

O coletor `locks` (cadeias de bloqueio de `sys.dm_exec_requests`, a cada 15 s). Um episódio por sessão bloqueadora enquanto uma sessão bloqueada espera pelo menos 30 s (`min_wait_s`).

## Por que importa

Sessões esperaram pelo menos 30 segundos outra sessão liberar seus locks. Enquanto dura, os usuários veem timeouts e telas travadas.

## Próxima ação

Comece pelo bloqueador do episódio mais longo. Um bloqueador em status "sleeping" com transações abertas costuma ser uma aplicação que esqueceu o commit; um que está executando é uma query ou rotina longa para otimizar ou tirar do horário de pico.

## Como validar

Grave de novo no mesmo horário depois da correção. Os episódios desse bloqueador devem sumir ou ficar bem mais curtos.

## Limitações

As amostras são feitas a cada 15 segundos, então bloqueios mais curtos podem passar despercebidos, e a duração de um episódio tem precisão de cerca de 15 segundos.
