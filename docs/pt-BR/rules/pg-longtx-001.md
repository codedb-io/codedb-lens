# PG-LONGTX-001 · Transações longas ou sessões "idle in transaction"

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Confirmado | `pg_monitor` (`pg_read_all_stats`) | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

Sessões de clientes em `pg_stat_activity` com transação aberta há mais de 5 minutos (`max_tx_s: 300`), ou em
`idle in transaction` há mais de 60 segundos (`idle_tx_s: 60`). Mostra quem (usuário, aplicação, endereço) e a última consulta.

## Por que importa

A transação aberta mais antiga define o horizonte do vacuum **no cluster inteiro**: linhas mortas mais novas que ela não
podem ser removidas, e tabelas e índices incham. Ela também pode segurar locks que bloqueiam outras sessões.

## Próxima ação

Encontre a aplicação de cada `pid`. `idle in transaction` quase sempre é um `COMMIT` esquecido ou uma conexão devolvida ao
pool no meio da transação. Como rede de proteção:

```sql
ALTER SYSTEM SET idle_in_transaction_session_timeout = '10min';
SELECT pg_reload_conf();
```

## Limitações

Um retrato só vê as transações abertas no momento da verificação.
