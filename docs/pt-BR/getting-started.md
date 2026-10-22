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
`--sslmode` (PostgreSQL), `--no-update-check`.

## 4. Leia o relatório

Os relatórios ficam em HTML e JSON em `Documentos\codeDB Lens\reports\<perfil>` (Windows) ou
`~/codedb-lens/reports/<perfil>` (Linux). Veja [como ler a nota e a cobertura](index.md#como-ler-o-relatorio).
Cada achado leva à página da regra, com o raciocínio, os limites e o que fazer.

## 5. Use em CI ou num job agendado

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
