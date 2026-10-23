# PG-VERSION-002 · Minor atrás da última release

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Confirmado (minor atrás) · Indício (minor mais nova que a tabela embutida) | `public` | PostgreSQL 14–18, todos os serviços |

## O que verifica

A minor instalada (`server_version_num % 100`) contra a última minor do mesmo major embutida no Lens (14.24, 15.19,
16.15, 17.11, 18.6, todas lançadas em 2026-08-13). Mostra a versão instalada, a última versão e sua data de lançamento, e
quantas minors você está atrás.

## Por que importa

As releases minor corrigem problemas de segurança, corrupção de dados e bugs frequentes, e são atualizações diretas
dentro do mesmo major. Ficar atrás mantém problemas conhecidos em aberto.

## Próxima ação

Atualize para a última release minor: troca de binários e restart, sem dump nem `pg_upgrade`. Leia as notas de release das
minors que você pula.

## Como validar

Depois do restart, rode `SHOW server_version` e confirme que a nova minor é a desejada. Veja no log do servidor se não há erros na subida, rode `ALTER EXTENSION nome UPDATE` se as notas de release pedirem e confirme que as réplicas também foram atualizadas. Rode o codeDB Lens de novo: a regra deve ficar OK.

## Limitações

- A tabela vem desta versão do Lens; atualize o Lens para renová-la.
- Uma minor mais nova que a tabela é reportada como *Indício* (a tabela pode estar desatualizada), nunca como OK.
- Serviços gerenciados costumam atrasar em relação às releases da comunidade; se o provedor não tem minor mais nova, ainda não há o que aplicar.
- Majors fora da tabela (13 e anteriores, ou ainda não listados) não são verificados; [PG-VERSION-001](pg-version-001.md) reporta o fim do suporte.
