# PG-SLOTS-001 · Slots de replicação retendo WAL, ou réplicas atrasadas

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Indício · Confirmado acima de 10 GB retidos ou `wal_status` lost/unreserved | `pg_monitor` | PostgreSQL 14+, RDS, Aurora, Azure Flexible (no primário) |

## O que verifica

- `pg_replication_slots`: slots **inativos** retendo pelo menos 1 GB de WAL (`retained_mb`), ou qualquer slot com
  `wal_status` `unreserved` ou `lost`.
- `pg_stat_replication`: réplicas com `replay_lag` de pelo menos 60 s (`lag_s`).

## Por que importa

Um slot guarda todo segmento de WAL que o consumidor ainda não confirmou. Se o consumidor sumiu (uma réplica antiga, uma
ferramenta de CDC desligada), o WAL se acumula até encher o disco e o primário parar de aceitar escritas.

## Próxima ação

```sql
-- slot que não é mais usado
SELECT pg_drop_replication_slot('<nome_do_slot>');
```

Para réplicas atrasadas, verifique disco e CPU da réplica, a rede, e consultas longas na réplica que atrasam o replay
(`max_standby_streaming_delay`). Considere `max_slot_wal_keep_size` como limite de segurança.

## Como validar

Rode `SELECT slot_name, active, pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn)) AS retained FROM pg_replication_slots`: o slot removido deve ter sumido e os demais devem estar ativos com retenção pequena. Nas réplicas, `SELECT application_name, replay_lag FROM pg_stat_replication` deve ficar abaixo do limite. O diretório de WAL encolhe no próximo checkpoint, não na hora.

## Limitações

Só faz sentido no primário. Um slot pode ficar inativo por alguns segundos enquanto o consumidor reconecta.
