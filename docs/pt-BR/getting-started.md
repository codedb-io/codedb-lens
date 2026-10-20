# Primeiros passos

## 1. Crie um login somente leitura

Use os scripts em [Permissões mínimas](permissions.md). O Lens nunca escreve no seu banco, e o login não precisa de mais
que isso.

## 2. Instale

- **Windows:** execute `codedb-lens_<versão>_windows_amd64_setup.exe`. Ele instala só para o seu usuário (sem admin) e
  coloca o **codeDB Lens** no menu Iniciar. Também há um `.zip` portátil.
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
