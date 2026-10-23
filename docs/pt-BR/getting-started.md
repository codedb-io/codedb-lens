# Primeiros passos

## 1. Crie um login somente leitura

Use os scripts em [Permissões mínimas](permissions.md). O Lens nunca escreve no seu banco, e o login não precisa de mais
que isso.

## 2. Instale

Baixe os arquivos em [GitHub Releases](https://github.com/codedb-io/codedb-lens/releases/latest) ou na
[página do codeDB Lens](https://app.codedb.com.br/lens?utm_source=lens&utm_medium=docs).

- **Windows:** execute `codedb-lens_windows_amd64_setup.exe`. Ele instala só para o seu usuário (sem admin) e
  coloca o **codeDB Lens** no menu Iniciar. Também há um `.zip` portátil. O instalador ainda não tem assinatura de
  código, então o SmartScreen do Windows pode avisar: escolha **Mais informações → Executar assim mesmo**.
- **Linux (x64 / arm64):** extraia o `.tar.gz` e coloque o `codedb-lens` no `PATH`.

## 3. Rode

### App (Windows)

1. Abra o **codeDB Lens** e escolha **SQL Server** ou **PostgreSQL**.
2. Preencha o servidor (ou cole uma connection string) e o login.
3. **Testar conexão**, depois **Rodar diagnóstico**. O relatório abre no app; a conexão fica salva para a próxima vez e a
   senha vai para o Gerenciador de Credenciais do Windows.

### Linha de comando (Windows e Linux)

```bash
# SQL Server, autenticação Windows
codedb-lens check --engine mssql --server sql01 --auth windows

# SQL Server, login SQL (senha pela variável de ambiente)
CODEDB_LENS_PASSWORD='…' codedb-lens check --engine mssql --server sql01,1433 --user codedb_lens

# PostgreSQL, connection string numa variável de ambiente
PG_DSN='postgres://codedb_lens:…@pg01:5432/postgres' codedb-lens check --engine pg --dsn-env PG_DSN

# Salve as configurações como perfil e reutilize
codedb-lens check --engine mssql --server sql01 --auth windows --save prod
codedb-lens check --profile prod
```

Flags úteis: `--lang en|pt-BR`, `--out <pasta>`, `--trust-server-cert` (certificado autoassinado no SQL Server),
`--sslmode` (PostgreSQL), `--redact` (veja [abaixo](#5-compartilhe-um-relatorio-com-seguranca)), `--no-update-check`.

## 4. Leia o relatório

Os relatórios ficam em HTML e JSON em `Documentos\codeDB Lens\reports\<perfil>` (Windows) ou
`~/codedb-lens/reports/<perfil>` (Linux). Veja [como ler a nota e a cobertura](index.md#como-ler-o-relatorio).
Cada achado leva à página da regra, com o raciocínio, os limites e o que fazer.

O relatório HTML abre na aba **Resumo**: nota e cobertura, a maior severidade em aberto, os 3 principais riscos (por
severidade, depois confirmados antes de indícios) e a evolução em relação à execução anterior da mesma instância na
mesma pasta. A evolução exige a [chave grátis](compare.md), como o `compare`; sem ela o resumo mostra um convite curto,
e a primeira execução diz "Primeiro diagnóstico". **Imprimir resumo** imprime só essa aba em uma página A4 (use o
imprimir como PDF do navegador para enviar a um gestor ou cliente). A aba **Técnico** traz o relatório completo, e cada
achado tem uma nota **Como validar a correção**. O JSON ganha o campo `evolution`.

## 5. Compartilhe um relatório com segurança

Os relatórios contêm nomes de servidor, bancos, objetos e logins. Para enviar a um consultor, fornecedor ou cliente,
acrescente `--redact` (só na linha de comando):

```bash
codedb-lens check --profile prod --redact
```

Com `--redact`, **só** `<timestamp>.redacted.html` e `.redacted.json` são gravados; o relatório sem máscara não é salvo.

- **Mascarados** com tokens estáveis como `db-3fa91c07`: host, bancos (os bancos de sistema continuam visíveis),
  schemas, tabelas, índices, logins, usuários, aplicações, endereços de cliente, jobs do SQL Agent, slots de replicação
  e caminhos de arquivo. O mesmo nome recebe sempre o mesmo token no relatório, então a evidência continua coerente.
- **Removidos:** textos de consulta e mensagens de passos do SQL Agent viram `[redacted]`. Mensagens de erro brutas
  recebem a mesma máscara.
- **Não mascarados:** versões, números, severidades, datas, o SQL da regra e nomes de bancos de sistema.
- **Salt:** os tokens são um HMAC com um salt aleatório por máquina, guardado em `redact.salt` ao lado do
  `profiles.json`, então não dá para reverter chutando nomes. A mesma máquina gera os mesmos tokens a cada execução, o
  que mantém o [`compare`](compare.md) funcionando; execuções mascaradas só se comparam com mascaradas. Apague o
  arquivo para trocar os tokens.
- **Atenção:** a máscara cobre as colunas das regras embutidas. Uma regra própria carregada com `--rules-dir` que use
  outros nomes de coluna, ou texto livre, não é mascarada. Revise o arquivo antes de compartilhar.

Nada é enviado, com ou sem `--redact`. O HTML mostra um selo "mascarado" e o JSON tem `redacted: true`.
## 6. Use em CI ou num job agendado

O `check` termina com um código que o pipeline pode usar. Por padrão, achados nunca reprovam a execução; defina uma
política:

| Código | Significado |
|---|---|
| `0` | Execução concluída e a política passou |
| `1` | Um achado com severidade igual ou maior que `--fail-on` (`critical`, `high`, `medium`, `low`) |
| `2` | Erro de conexão, configuração ou execução. Nunca vira achado do banco |
| `3` | Cobertura abaixo de `--min-coverage` (porcentagem) |

Um achado tem prioridade sobre a cobertura baixa. O relatório JSON fica ao lado do HTML, com o mesmo nome.

```yaml
# GitHub Actions
- name: codeDB Lens
  env:
    CODEDB_LENS_PASSWORD: ${{ secrets.LENS_PASSWORD }}
    CODEDB_LENS_NO_UPDATE: "1"
  run: >
    codedb-lens check --engine pg --server db.internal --user codedb_lens
    --out lens-report --fail-on high --min-coverage 80
```

Rodando o Lens agendado para acompanhar a produção? É isso que o [codeDB](https://codedb.com.br/?utm_source=lens&utm_medium=docs&utm_campaign=ci)
faz 24 horas por dia, com histórico e alertas.
