# PG-WRAPAROUND-001 · Risco de wraparound de XID

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Crítica | Indício (idade ≥ 1,5 × `autovacuum_freeze_max_age`) · Confirmado (idade ≥ 1 bilhão) | `public` | Todas as versões e serviços PostgreSQL |

## O que verifica

`age(datfrozenxid)` de cada banco que aceita conexões.

## Por que importa

Perto de 2³¹ (≈ 2,1 bilhões) de IDs de transação, o PostgreSQL deixa de aceitar escritas para proteger os dados. A recuperação exige um VACUUM longo.
Uma idade bem acima de `autovacuum_freeze_max_age` indica que o autovacuum está ficando para trás.

## Próxima ação

```sql
-- No banco afetado: as tabelas mais antigas
SELECT c.oid::regclass, age(c.relfrozenxid) FROM pg_class c
WHERE c.relkind IN ('r', 'm', 't') ORDER BY 2 DESC LIMIT 20;
```

Rode `VACUUM (FREEZE, VERBOSE)` nelas e procure o que trava o autovacuum: transações longas,
slots de replicação abandonados, prepared transactions antigas.

## Limitações

Um retrato mostra a idade atual, não a velocidade de consumo. Quantos dias faltam exige histórico.
