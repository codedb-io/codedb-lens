# MSSQL-VERSION-001 · Versão fora de suporte

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Confirmado (suporte encerrado) · Indício (encerra em até 365 dias) | `public` | SQL Server 2012–2022, RDS (não Azure SQL Database / MI, que não têm versão) |

## O que verifica

`SERVERPROPERTY('ProductMajorVersion')` contra a data de fim do suporte estendido da Microsoft embutida no Lens:

| Versão | Fim do suporte estendido |
|---|---|
| SQL Server 2014 | 2024-07-09 |
| SQL Server 2016 | 2026-07-14 |
| SQL Server 2017 | 2027-10-12 |
| SQL Server 2019 | 2030-01-08 |
| SQL Server 2022 | 2033-01-11 |

## Por que importa

Versões fora de suporte não recebem correções de segurança. Um upgrade de versão leva meses de planejamento e testes.

## Próxima ação

Planeje o upgrade para uma versão suportada: verifique a compatibilidade das aplicações e o compatibility level de cada banco.

## Limitações

- As datas vêm da tabela desta versão do Lens; atualize o Lens para renová-las.
- A atualização de CU (cumulative update) ainda não é verificada.
