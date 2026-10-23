# PG-VERSION-001 · Versão fora de suporte

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Confirmado (EOL passado) · Indício (EOL em até 365 dias) | `public` | Todas as versões e serviços PostgreSQL |

## O que verifica

`server_version_num` contra as datas de fim de vida da comunidade embutidas no Lens (14: 2026-11-12, 15: 2027-11-11,
16: 2028-11-09, 17: 2029-11-08, 18: 2030-11-14).

## Por que importa

Versões fora de suporte não recebem correções de segurança nem de bugs. Upgrades de major exigem planejamento e testes.

## Próxima ação

Planeje o upgrade de major: `pg_upgrade`, replicação lógica ou o caminho de upgrade do seu provedor de nuvem.

## Como validar

Depois do upgrade, rode `SHOW server_version` e confirme que a versão major está dentro do suporte da comunidade. Em seguida rode `ANALYZE` (o pg_upgrade não preserva as estatísticas do planner), confira as extensões com `SELECT extname, extversion FROM pg_extension` e deixe a aplicação passar por um ciclo completo antes de remover o cluster antigo.

## Limitações

- Provedores de nuvem podem oferecer suporte estendido pago além da data da comunidade.
- A atualização de versão minor é verificada por [PG-VERSION-002](pg-version-002.md).
