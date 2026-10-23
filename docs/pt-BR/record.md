# Gravar ao longo do tempo

O `check` é um retrato: só vê um bloqueio ou uma query lenta se estiver acontecendo naquele momento. O `record`
acompanha a instância por um período, a cada 15 segundos, e o `report` transforma a gravação num relatório do que
aconteceu: episódios de bloqueio, picos de espera, queries longas, conexões perto do limite e as queries mais pesadas
do período.

- **1 hora** sem chave, **24 horas** com a [chave grátis](compare.md#obter-a-chave-gratis).
- Mesmo login somente leitura e mesmas permissões do `check` ([Permissões](permissions.md)). Nada é instalado ou criado
  no servidor: nenhuma sessão de Extended Events, tabela ou job.
- Tudo fica na sua máquina, na pasta da gravação.

## Gravar

**App:** selecione uma conexão, abra a aba **Gravar**, escolha 1 ou 24 horas e clique em **Iniciar gravação**. Mantenha
o app aberto: fechar para a gravação (o que foi gravado é mantido). O Windows não entra em suspensão enquanto grava.

**Linha de comando:**

```bash
codedb-lens record --engine mssql --server prod01 --auth windows --duration 24h
codedb-lens record --engine pg --dsn-env PG_DSN --duration 1h --out ./noite-de-02-11
```

As flags de conexão são as mesmas do `check`. Sem `--out`, a pasta é `./codedb-lens-record-<host>-<data>`. O progresso
aparece uma vez por minuto. **Ctrl+C** para de forma segura, assim como `SIGTERM` (`docker stop`, systemd).

A gravação também para sozinha no fim da duração, ou se chegar a **500 MB** em disco (uma gravação de 24 horas costuma
ficar bem abaixo disso). Só uma gravação roda por vez num computador.

Se a conexão cair, a gravação continua e reconecta sozinha (depois de 15 s, 30 s e então a cada minuto). O tempo sem
dados aparece como **Sem dados** no relatório, nunca como OK. O prazo final não muda.

### Num servidor, com Docker

Uma gravação de 24 horas é mais fácil numa máquina que fica ligada. A imagem não tem shell e só grava na pasta que você
montar:

```bash
docker run -d --name lens-rec -e CODEDB_LENS_PASSWORD -v "$PWD:/out" \
  ghcr.io/codedb-io/codedb-lens record --engine pg --server db01 --user codedb_lens --duration 1h --out /out/rec
docker stop lens-rec    # para antes, de forma segura
```

A chave grátis fica no cofre de credenciais do computador, então dentro de um container o `record` roda até 1 hora.
Para 24 horas, use o binário no próprio host (abaixo), com a chave importada nele.

### Num servidor Linux, com systemd

```ini
# /etc/systemd/system/lens-record.service
[Service]
Type=simple
User=lens
Environment=CODEDB_LENS_PASSWORD=troque-me
ExecStart=/usr/bin/codedb-lens record --engine mssql --server db01 --user codedb_lens --duration 24h --out /var/lib/lens/rec-%i
```

`systemctl start lens-record` inicia; `systemctl stop lens-record` para de forma segura. No app, **Copiar comando para o
servidor** dá a linha do `record` da conexão selecionada, sem a senha.

## Relatório

```bash
codedb-lens report ./codedb-lens-record-prod01-20261102-2200
codedb-lens report ./codedb-lens-record-prod01-20261102-2200 --from 02:00 --to 03:30
```

Gera `report.html` e `report.json` na pasta da gravação (`report-<de>_<até>` para um período). Funciona com uma gravação
ainda em andamento ou interrompida. No app, clique numa gravação da lista, ou em **Ver relatório** durante a gravação.

- `--from` / `--to`: `15:04` (o primeiro horário assim depois do início da gravação, então `02:00` numa gravação noturna
  é no dia seguinte), `2006-01-02 15:04` ou RFC 3339. Horários no fuso deste computador.
- `--redact`: mascara host, bancos, logins e aplicações e remove os textos das queries, como o `check --redact`.
- `--lang en|pt-BR`. O relatório leva os dois idiomas; o botão no topo alterna.

### O que o relatório mostra

- **Tempo com dados**: parte do período com amostras. É o selo de cobertura de uma gravação: abaixo de 100% significa
  que a conexão caiu, o computador suspendeu ou a gravação parou.
- **Achados** das regras de série, cada linha um episódio (início, fim, amostras, pico e a amostra do pico, com o texto
  da query quando houver):

| SQL Server | PostgreSQL | Achado |
|---|---|---|
| [MSSQL-REC-BLOCKING-001](rules/mssql-rec-blocking-001.md) | [PG-REC-BLOCKING-001](rules/pg-rec-blocking-001.md) | bloqueios de 30 s ou mais |
| [MSSQL-REC-WAITS-001](rules/mssql-rec-waits-001.md) | [PG-REC-WAITS-001](rules/pg-rec-waits-001.md) | picos de espera por 30 s ou mais |
| [MSSQL-REC-LONGQUERY-001](rules/mssql-rec-longquery-001.md) | [PG-REC-LONGQUERY-001](rules/pg-rec-longquery-001.md) | queries executando por 1 minuto ou mais |
| [MSSQL-REC-WORKERS-001](rules/mssql-rec-workers-001.md) | [PG-REC-CONN-001](rules/pg-rec-conn-001.md) | threads de trabalho / conexões em 90% do limite |

- **Gráficos ao longo do tempo**: requisições ou sessões ativas, sessões bloqueadas, esperas, vazão (batch requests ou
  commits por segundo), page life expectancy, threads de trabalho ou conexões. Faixas cinza: sem dados. Faixas
  vermelhas: episódios de um achado.
- **Top queries do período**, por CPU (SQL Server) ou tempo de execução (PostgreSQL).

Todo número técnico tem um botão **i** que explica o que significa e como ler. Uma gravação não tem nota A–F: a nota é
do `check`.

## O que é coletado

| A cada 15 s | A cada 5 min |
|---|---|
| SQL Server: requisições ativas, cadeias de bloqueio, tempo de espera por tipo, batch requests, compilações, page life expectancy, threads de trabalho | CPU, duração e leituras por query (`sys.dm_exec_query_stats`), top 50 |
| PostgreSQL: sessões ativas, esperas por lock, commits, rollbacks, leituras, bytes temporários e deadlocks por banco, conexões | tempo, chamadas e leituras por query (`pg_stat_statements`), top 50 |

Os textos das queries são guardados (até 4 KB) só na pasta da gravação. Sem `pg_stat_statements` as top queries ficam
**Sem dados**; o resto funciona. As consultas de coleta são dados, como as regras: `rules/<engine>/_collect/` no
repositório.

## Overhead

Meta: menos de 1% da CPU do servidor. Medido com `tools/overhead` (carga OLTP sintética, gravação no meio de dois
períodos sem gravação), nos containers de teste:

| | SQL Server 2022 | PostgreSQL 17 |
|---|---|---|
| Parte da CPU do servidor usada pelo gravador | **0,003%** | **0,0001%** (tempo de execução, um teto) |
| Tempo de servidor do gravador por hora | cerca de 1,9 s | cerca de 0,06 s |
| Vazão da carga sem / com gravação | 1953 / 1896 ops/s | 6879 / 6548 ops/s |
| Tamanho da gravação nestes servidores de teste | cerca de 0,04 MB por hora | cerca de 0,04 MB por hora |

Medido em 2026-10-23 numa estação de trabalho (20 núcleos, Docker Desktop), 10 minutos de gravação entre dois períodos
de 5 minutos sem gravação, os dois bancos ao mesmo tempo. Nessa máquina o gerador de carga, os bancos e o Lens dividem a
CPU, e a vazão variou de +7% a −5% entre rodadas com e sem gravação, então o tempo de servidor das consultas do próprio
gravador é a medida direta. Um servidor movimentado grava mais por hora (mais sessões e queries por amostra).

O workflow `Overhead` repete a medição no GitHub Actions (30 minutos de gravação).
