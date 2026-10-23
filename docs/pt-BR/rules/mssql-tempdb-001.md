# MSSQL-TEMPDB-001 · Arquivos do TempDB fora do padrão para concorrência

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Confirmado | `VIEW ANY DEFINITION` + `VIEW SERVER STATE` | SQL Server 2017+, Azure SQL MI, RDS |

## O que verifica

A partir de `sys.master_files` (banco 2) e `sys.dm_os_sys_info`:

- Menos arquivos de dados que CPUs lógicas, até 8.
- Arquivos de dados com tamanhos diferentes (o SQL Server preenche proporcionalmente, então o maior recebe a maior parte da carga).
- Qualquer arquivo do TempDB crescendo em percentual.

## Por que importa

O TempDB é compartilhado pela instância inteira. Poucos arquivos, ou de tamanhos diferentes, concentram a alocação em
um só e causam esperas `PAGELATCH_*` nas páginas de alocação; crescimento em percentual deixa cada crescimento maior e mais lento.

## Próxima ação

```sql
ALTER DATABASE tempdb MODIFY FILE (NAME = tempdev, SIZE = 8GB, FILEGROWTH = 512MB);
ALTER DATABASE tempdb ADD FILE (NAME = temp2, FILENAME = '<caminho>\tempdb2.ndf', SIZE = 8GB, FILEGROWTH = 512MB);
```

Reduções de tamanho só valem depois de um restart.

## Como validar

Liste os arquivos com `SELECT name, size * 8 / 1024 AS size_mb, growth, is_percent_growth FROM tempdb.sys.database_files`: os arquivos de dados devem ter o mesmo tamanho, crescimento fixo em MB (`is_percent_growth = 0`) e um arquivo por CPU lógica, até 8. Arquivos novos e mudanças de crescimento valem na hora; reduzir ou redimensionar arquivos existentes pode exigir restart. Depois acompanhe as esperas PAGELATCH no tempdb.

## Limitações

Oito arquivos é um ponto de partida; contenção pesada pode exigir mais, em múltiplos de 4.
