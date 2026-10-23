# PG-LOCKS-001 · Sessões bloqueadas por locks neste momento

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Confirmado | `pg_monitor` (`pg_read_all_stats`) | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

Sessões em `pg_stat_activity` esperando um lock (`wait_event_type = 'Lock'`) há pelo menos 5 segundos (`min_wait_s`), com
o primeiro bloqueador de `pg_blocking_pids()`: estado, usuário, aplicação e idade da transação.

## Por que importa

Locks fazem fila: um único `ALTER TABLE` esperando atrás de uma transação aberta faz todas as consultas seguintes naquela
tabela esperarem também.

## Próxima ação

- Bloqueador `idle in transaction`: uma aplicação deixou a transação aberta. Corrija a aplicação; configure
  `idle_in_transaction_session_timeout` como rede de proteção.
- Bloqueador `active`: um comando longo. Como último recurso, `SELECT pg_cancel_backend(<pid>)` (cancela a consulta) antes
  de `pg_terminate_backend(<pid>)` (fecha a sessão).
- Para DDL em produção, use `SET lock_timeout = '3s'` para a migração desistir em vez de travar a tabela.

## Como validar

Rode de novo `SELECT pid, pg_blocking_pids(pid), wait_event FROM pg_stat_activity WHERE wait_event_type = 'Lock'`. O resultado deve vir vazio (ou com esperas curtas) quando o bloqueador der commit, rollback ou for encerrado. Repita nos horários de pico: se a mesma aplicação continua bloqueando, a causa está no tratamento de transações dela, não nessa sessão.

## Limitações

Um retrato só vê bloqueios no momento da verificação.
