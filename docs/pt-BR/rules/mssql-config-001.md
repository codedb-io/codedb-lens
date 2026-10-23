# MSSQL-CONFIG-001 · Configurações da instância no padrão

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Indício | `VIEW SERVER STATE` (CPU e RAM do host) | SQL Server 2017+, Azure SQL MI, RDS |

## O que verifica

| Configuração | Quando aparece | Ponto de partida sugerido |
|---|---|---|
| `max server memory (MB)` | Ainda no padrão (2147483647) | ~80% da RAM do host |
| `max degree of parallelism` | `0` em hosts com mais de 8 CPUs lógicas (`maxdop_cpus`) | `8` |
| `cost threshold for parallelism` | Ainda no padrão `5` | `50` |
| `optimize for ad hoc workloads` | `0` | `1` |

## Por que importa

Com memória ilimitada, o SQL Server pode sufocar o sistema operacional e outros serviços. Com o paralelismo padrão,
consultas baratas rodam em paralelo e disputam CPU (esperas `CXPACKET` / `CXCONSUMER`).

## Próxima ação

```sql
EXEC sp_configure 'show advanced options', 1; RECONFIGURE;
EXEC sp_configure 'max server memory (MB)', <valor>; RECONFIGURE;
```

Mude uma configuração por vez e observe o efeito.

## Como validar

Confira o valor em uso, não só o configurado: `SELECT name, value, value_in_use FROM sys.configurations WHERE name IN ('max server memory (MB)', 'max degree of parallelism', 'cost threshold for parallelism', 'optimize for ad hoc workloads')`. O `value_in_use` deve mostrar o número novo (rode `RECONFIGURE` se não mostrar). Depois acompanhe CPU e memória por alguns dias.

## Limitações

São pontos de partida comuns, não respostas universais. Hosts com outros serviços (SSIS, SSRS, aplicações) precisam de
mais memória livre para o sistema operacional.
