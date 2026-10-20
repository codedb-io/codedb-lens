# MSSQL-BLOCKING-001 · Sessões bloqueadas neste momento

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Confirmado | `VIEW SERVER STATE` | SQL Server 2017+, Azure SQL MI, RDS, Azure SQL Database |

## O que verifica

`sys.dm_exec_requests` com `blocking_session_id`, bloqueadas há pelo menos 5 segundos (`min_wait_ms: 5000`), com status,
login, host, programa e transações abertas de quem bloqueia, a partir de `sys.dm_exec_sessions`.

## Por que importa

Sessões bloqueadas ficam paradas até o bloqueador liberar os locks. Os usuários veem timeouts; cadeias longas podem
esgotar as threads de trabalho (`THREADPOOL`).

## Próxima ação

- Bloqueador **sleeping** com `blocker_open_trans > 0`: uma aplicação abriu transação e não fez commit. Corrija a
  aplicação; numa emergência, fale com o responsável antes de `KILL <sessão>`.
- Bloqueador **running**: uma consulta ou carga longa. Veja o plano e se pode rodar fora do horário de pico.

## Limitações

Um retrato só vê o que está bloqueado no momento da verificação. Um bloqueio que aconteceu às 03:00 não aparece aqui.
