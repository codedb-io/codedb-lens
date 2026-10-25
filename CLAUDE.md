# codeDB Lens

Ferramenta gratuita de diagnóstico para SQL Server e PostgreSQL. É o lead magnet do codeDB
(SaaS de monitoramento, Azure, Stripe): útil de verdade, leva o usuário ao SaaS e captura e-mail.
Um binário `codedb-lens` (engine por `--engine mssql|pg`); "for SQL Server" e "for PostgreSQL" são
páginas de docs/marketing. Windows (GUI + CLI) e Linux (CLI).
Repo: github.com/codedb-io/codedb-lens (privado até a v0.5, escrito como público, em inglês).
Plano detalhado: `planos/01-plano-inicial.md`. Handoffs de sessão em `handoff/` (o mais recente primeiro). Ambas as pastas são locais, fora do git.

Proposta completa, matriz de validação e análise: https://claude.ai/code/artifact/8dc89e78-12ee-4ac4-b670-5f07795a99c4

## Invariantes (não quebrar)

- **Somente leitura** no banco do usuário: `check`, `record`, `report` e `compare` são sempre somente leitura, e nenhum
  agente é obrigatório. O `mcp` é somente leitura por padrão; escrever no banco só com o opt-in duplo da Q44 (perfil
  com `mcp allow-write`, digitado num terminal, + `--allow-write` no cliente), divulgado apenas nas docs
  (`mcp-write`), nunca no README, na GUI ou no site. As permissões mínimas do Lens não mudam.
- **Sem telemetria.** Nenhum dado do banco ou do usuário sai da máquina. Chamadas de rede: checagem
  de versão ≤ 1×/dia (GET anônimo ao GitHub Releases), desligável (`--no-update-check`, `CODEDB_LENS_NO_UPDATE=1`),
  e os POSTs de cadastro e renovação da chave grátis (`internal/license`), só por ação explícita do usuário. Licença verificada offline.
- **Regras são dados** (SQL + YAML), não código. Motor + um adaptador por banco. Trocar o motor
  não pode exigir reescrever regras.
- Todo achado tem: ID, título, severidade, confiança (Confirmado / Indício / Sem dados),
  evidência, impacto, próxima ação, link de documentação.
- Nota A–F **sempre** com selo de cobertura (% de regras executadas). Regra sem permissão ou
  sem dados vira "Sem dados", nunca falso "OK".
- Limitar continuidade/escala (24h, 1 instância por chave), **nunca** a qualidade do diagnóstico.
- **Não reaproveitar código do agente do codeDB** (repo MonitorSQL). Este repo será público.
- Permissões mínimas: SQL Server `VIEW SERVER STATE` + `VIEW ANY DEFINITION` + `CONNECT ANY DATABASE` + leitura no msdb;
  PostgreSQL `pg_monitor` + `CONNECT` em cada banco. Regra que precise de mais = rever a regra.

## Stack (MVP)

- **Go** (última estável). Drivers Go puro: `microsoft/go-mssqldb`, `jackc/pgx/v5`. CLI com `flag` da stdlib.
- GUI só no Windows: Wails v2 (WebView2), HTML/JS puro sem npm; o mesmo `.exe` vira CLI quando recebe
  argumentos. Linux: só CLI (servidores/Docker).
- Relatório: `html/template`, HTML autônomo (CSS/JS inline, troca en/pt-BR) + JSON. Sem servidor web local.
- Regras embutidas com `go:embed` em `rules/<engine>/<ID>/{rule.yaml,query.sql}`; `--rules-dir` sobrepõe.
- Perfis em `%APPDATA%\codeDB Lens\profiles.json` / `~/.config/codedb-lens/`; senha só no cofre do SO (`go-keyring`).
- i18n: `en` + `pt-BR`; textos das regras inline no YAML; CI falha se faltar tradução.
- Atualização: `codedb-lens update` verifica `checksums.txt` assinado em Ed25519 (chave pública embutida).
  Só funciona com o repo público (fase 2).
- Segurança das queries: timeout 10 s por regra, execução sequencial, `application_name="codeDB Lens"`,
  MSSQL `LOCK_TIMEOUT`/`DEADLOCK_PRIORITY LOW`, PG `statement_timeout`/`lock_timeout`/read-only.
- Links do SaaS só em `internal/saas/links.go`: landing `https://codedb.com.br/`, cadastro
  `https://app.codedb.com.br/auth/register`, sempre com UTM `utm_source=lens`.
- Ícone: `assets/icon/lens-icon.svg` (mestre) → `python assets/icon/generate.py`. `img/` é só referência
  da marca do SaaS (não versionada).
- Licença Apache 2.0.
- Compatibilidade alvo: SQL Server 2017–2025 (Win/Linux), Azure SQL MI, RDS; PostgreSQL 15–18,
  RDS, Aurora, Azure Flexible. SQL 2016 e PG 14 best-effort.
- Docs: MkDocs Material + `mkdocs-static-i18n` (`docs/en`, `docs/pt-BR`), `mkdocs build --strict`.
- Distribuição: fase 1 GitHub Releases (Inno Setup por usuário + zip; tar.gz Linux x64/arm64) via GoReleaser;
  fase 2 winget, Chocolatey, .deb/.rpm, ghcr.io. Assinatura de código Windows antes da fase 2 (Azure Trusted Signing).
- Azure só a partir da fase 2 (chave Ed25519 assinada no backend do SaaS, chave privada no Key Vault).

## Roadmap

| Fase | Entrega | Critério de saída |
|---|---|---|
| 0 | Validação | Regras do MVP confirmadas, decisões fechadas |
| 1 | v0.1 `check`, 12 regras por banco (uso privado) | CI verde na matriz de versões, testado pelos 2 clientes |
| 2 | v0.5 `compare` + chave grátis (lançamento público) | Cadastro self-service do codeDB no ar |
| 3 | v0.8 `record` (24h) + regras v1 | Overhead medido e documentado |
| 4 | v1.0 `share` + `connect` | Primeiros trials vindos do Lens |

Status (2026-10-23): fase 3 concluída. Repo público, v0.8.0 publicada (`record` 1h/24h + `report` com linha do tempo,
8 regras de série `*-REC-*`, aba Gravar na GUI; overhead medido em `RELEASING.md`; plano em `planos/09-v0.8-record.md`;
v0.6: resumo executivo, `--redact`, `verify`,
`*-VERSION-002`; 26 regras; sem nota abaixo de 6 regras aplicáveis. v0.6.1: attestations, bucket Scoop em `bucket/`
atualizado pelo `publish.yml`; winget `codeDB.Lens` em revisão, sem `WINGET_TOKEN`. v0.7: GUI com tela inicial, abas
por conexão, botão "Atualizar agora", excluir do histórico; plano da v0.8 em `planos/08-v0.7-v0.8.md`), `.deb`/`.rpm`, imagem pública
`ghcr.io/codedb-io/codedb-lens`, docs em `https://lens.codedb.com.br/` (GitHub Pages, workflow `docs.yml`),
página `https://app.codedb.com.br/lens` no ar. Próximo marco: v1.0 `share` + `connect` (assinatura quando houver
certificado). Calibrar os limites das regras `*-REC-*` com gravações reais.

Chave grátis (contrato com o MonitorSQL, 2026-10-21): `POST https://api.codedb.com.br/lens/register` (sem staging) e
`POST /lens/renew` (2026-10-22; chave atual, mesmo vencida; janela −14 a +30 dias do vencimento),
validação estrita (campo novo entra primeiro no backend). Licença `base64url(payload).base64url(Ed25519 sobre o texto
do payload)`, kid `2026-1`, 90 dias, `max_instances` 1 (aplicado localmente). Consentimento em dois campos
(`privacy_ack` obrigatório, `marketing_opt_in` opcional) + `consent_version`; o texto fica em
`internal/license/consent.go`, versão `lens-2026-1` (aprovada pelo jurídico em 2026-10-22, igual ao rascunho `lens-2026-0`).

Em aberto: assinatura de código (SignPath Foundation recusou em 2026-10-22 por falta de visibilidade; reenviar com
tração. Azure Artifact Signing não atende o Brasil; Certum Open Source ~€49/ano quando houver verba).

---

## graphify

Quando existir `graphify-out/graph.json`:
- Para perguntas sobre o código, rode primeiro `graphify query "<pergunta>"`; `graphify path "<A>" "<B>"`
  para relações e `graphify explain "<conceito>"` para conceitos.
- Se `graphify-out/wiki/index.md` existir, use-o para navegação ampla.
- Leia `graphify-out/GRAPH_REPORT.md` só para revisão de arquitetura.
- Depois de alterar código, rode `graphify update .` (só AST, sem custo de API).

## Distribuição de modelos

- Opus: arquitetura, bugs complexos e revisão de código.
- Sonnet: edições, testes, documentação e refatoração.
- Haiku: pesquisas e resumos.
- Especifique o modelo em cada chamada de agente.

## Delegação

- Use a skill `/grilling` para fechar decisões antes de implementar.
- Um subagente por tarefa. Planeje antes de executar.
- Execute subagentes independentes em paralelo.
- Leia o relatório, nunca os arquivos.
