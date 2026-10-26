# Modo escrita da IA (MCP)

Por padrão, o servidor MCP (`codedb-lens mcp`) só lê: nenhuma ferramenta altera um banco. O modo escrita acrescenta uma
ferramenta, `execute_sql`, que deixa o seu cliente de IA rodar `UPDATE`, `INSERT`, `CREATE INDEX` e instruções
parecidas **depois que você confirma cada uma**. Ele fica desligado até você ligá-lo, de propósito, em dois lugares.

> **Use em bancos onde um erro é aceitável.** Quem escreve o SQL é a IA. Você vê antes de rodar, mas uma instrução
> confirmada faz o que diz. Comece num banco de teste, use um login que só mexe no necessário e mantenha backups.

## Como é protegido

- **Duas chaves.** A ferramenta só existe quando o perfil permite escrita **e** o cliente inicia o Lens com
  `--allow-write`. Faltando qualquer uma, o `execute_sql` nem aparece na lista.
- **Você confirma cada chamada.** O cliente de IA mostra o SQL, o perfil, o banco e se é uma prévia, e pergunta a você.
  Nada roda se você recusar ou fechar a pergunta.
- **Prévia primeiro.** `dry_run` é `true` por padrão: a instrução roda numa transação, a IA recebe o número de linhas
  afetadas e a transação é desfeita. Para aplicar, a IA chama de novo com `dry_run: false`, e você confirma de novo.
- **Sem prévia para o que uma transação não comporta:** `VACUUM`, `CREATE INDEX CONCURRENTLY`, `ALTER DATABASE`,
  `BACKUP`, `RESTORE`, `DBCC`. Exigem `dry_run: false` e a sua confirmação, e não podem ser desfeitos.
- **Sempre recusado**, mesmo com o modo escrita ligado: `DROP DATABASE`, `SHUTDOWN`, configurações do servidor
  (`sp_configure`, `ALTER SYSTEM`, `RECONFIGURE`), logins, roles, usuários e permissões
  (`CREATE/ALTER/DROP LOGIN|ROLE|USER`, `GRANT`, `REVOKE`, `DENY`), acesso ao sistema operacional e a arquivos
  (`xp_cmdshell`, `COPY ... PROGRAM`, `lo_import`, `pg_read_file`, `OPENROWSET`, `BULK INSERT`), código que roda fora
  do banco (assemblies CLR, jobs do SQL Agent, linked servers e foreign servers, scripts externos, extensões e
  linguagens não confiáveis), troca de identidade, confiança ou dono (`EXECUTE AS`, `SET ROLE`, `SECURITY DEFINER`,
  `TRUSTWORTHY`, `ALTER AUTHORIZATION`, `OWNER TO`), controle de transação (`COMMIT`, `ROLLBACK`: transformariam uma
  prévia numa alteração) e SQL dinâmico que esconderia qualquer um deles (`EXEC('...')`, `sp_executesql`, `DO` e
  `EXECUTE` do PostgreSQL). A checagem lê o texto inteiro, comentários e strings incluídos, então uma palavra num
  comentário também é recusada: rode essa no seu cliente SQL.
- **Registro de auditoria.** Cada tentativa (prévia, alteração, recusa, não confirmada) vira uma linha em
  `mcp-audit.jsonl`, ao lado do `profiles.json` (`%APPDATA%\codeDB Lens\` no Windows, `~/.config/codedb-lens/` nos
  outros): hora, perfil, banco, o SQL e o SHA-256 dele, modo, resultado, linhas afetadas e se foi desfeito. Cada
  execução é registrada antes de começar e de novo com o resultado; se o registro não puder ser gravado, nada roda.
- **O seu login é o limite real.** Use um perfil separado, com um login feito para isso. O login mínimo do Lens
  ([Permissões](permissions.md)) não escreve, e deve continuar assim. Uma stored procedure que a IA chama roda como o
  código dela manda: se ela fizer commit, a prévia não desfaz.
- **Ligar exige uma pessoa num terminal**, mas o Lens não impede que um programa rodando como você edite o
  `profiles.json` ou a configuração do cliente. Um cliente de IA que pode rodar comandos no seu computador também
  pode, então mantenha o modo escrita longe de clientes que você deixa rodar comandos sem supervisão.

## Ligar

### 1. Um login para escrita

**SQL Server:**

```sql
CREATE LOGIN lens_ai_write WITH PASSWORD = '<senha>';
USE app;
CREATE USER lens_ai_write FOR LOGIN lens_ai_write;
ALTER ROLE db_datareader ADD MEMBER lens_ai_write;
ALTER ROLE db_datawriter ADD MEMBER lens_ai_write;
-- só se a IA for criar índices:
ALTER ROLE db_ddladmin ADD MEMBER lens_ai_write;
```

**PostgreSQL:**

```sql
CREATE ROLE lens_ai_write LOGIN PASSWORD '<senha>';
GRANT CONNECT ON DATABASE app TO lens_ai_write;
GRANT USAGE ON SCHEMA public TO lens_ai_write;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO lens_ai_write;
```

Salve um perfil com ele no app (por exemplo `app-write`), separado do seu perfil de diagnóstico.

### 2. Permitir escrita nesse perfil

```bash
codedb-lens mcp allow-write --profile app-write
```

Sem o Lens no `PATH`, no PowerShell: `& "$env:LOCALAPPDATA\Programs\codeDB Lens\codedb-lens.com" mcp allow-write --profile app-write`.
Ele pede para você digitar o nome do perfil. A partir daí o app mostra um alerta na tela **IA (MCP)** enquanto algum
perfil permitir escrita.

### 3. Acrescentar `--allow-write` no cliente

Edite a configuração do cliente à mão (o `codedb-lens mcp install` nunca acrescenta isso, e rodá-lo de novo remove) e
acrescente `--allow-write` aos argumentos do Lens, de preferência limitado a esse perfil:

```json
{
  "mcpServers": {
    "codedb-lens": {
      "command": "C:\\Users\\voce\\AppData\\Local\\Programs\\codeDB Lens\\codedb-lens.exe",
      "args": ["mcp", "--profile", "app-write", "--allow-write"]
    }
  }
}
```

Reinicie o cliente. O `list_profiles` passa a mostrar `write_enabled: true` para esse perfil.

O seu cliente precisa conseguir pedir confirmação (*elicitation* do MCP); se não conseguir, toda escrita é recusada.
O `--allow-write=no-confirm` deixa esse cliente escrever só com a aprovação por ferramenta dele, se houver. Evite.

## Desligar

```bash
codedb-lens mcp revoke-write --profile app-write
```

Vale na próxima chamada, sem reiniciar o cliente. Tire também o `--allow-write` da configuração do cliente.
O `codedb-lens mcp status` lista os perfis que ainda permitem escrita.
