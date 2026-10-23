# MSSQL-VERSION-001 · Versão fora de suporte

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Confirmado (suporte encerrado) · Indício (encerra em até 365 dias) | `public` | SQL Server 2012–2025, RDS (não Azure SQL Database / MI, que não têm versão) |

## O que verifica

`SERVERPROPERTY('ProductMajorVersion')` contra a data de fim do suporte estendido da Microsoft embutida no Lens:

| Versão | Fim do suporte estendido |
|---|---|
| SQL Server 2014 | 2024-07-09 |
| SQL Server 2016 | 2026-07-14 |
| SQL Server 2017 | 2027-10-12 |
| SQL Server 2019 | 2030-01-08 |
| SQL Server 2022 | 2033-01-11 |
| SQL Server 2025 | 2036-01-07 |

## Por que importa

Versões fora de suporte não recebem correções de segurança. Um upgrade de versão leva meses de planejamento e testes.

## Próxima ação

Planeje o upgrade para uma versão suportada: verifique a compatibilidade das aplicações e o compatibility level de cada banco.

## Como validar

Depois do upgrade, rode `SELECT SERVERPROPERTY('ProductVersion'), SERVERPROPERTY('ProductLevel')` e confirme que a versão major ainda está dentro do suporte. Confira cada banco com `SELECT name, compatibility_level FROM sys.databases` e deixe a aplicação passar por um ciclo completo (incluindo rotinas de fechamento de mês) antes de desligar o servidor antigo.

## Limitações

- As datas vêm da tabela desta versão do Lens; atualize o Lens para renová-las.
- A atualização de CU (cumulative update) é verificada por [MSSQL-VERSION-002](mssql-version-002.md).
