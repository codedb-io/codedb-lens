# Segurança e privacidade

- **Somente leitura.** Apenas consultas a catálogo, DMVs e estatísticas. As sessões usam timeouts de
  lock e de comando e se identificam como `codeDB Lens`.
- **Sem telemetria.** Nada sobre seu banco, seus servidores ou seu uso sai da sua máquina.
- **Uma chamada de rede opcional:** no máximo uma vez por dia, uma requisição anônima ao GitHub Releases
  para verificar se há versão nova. Desligue com `--no-update-check` ou `CODEDB_LENS_NO_UPDATE=1`.
- **Cadastro da chave grátis, só quando você pede:** o `codedb-lens register` (ou **Ativar chave grátis** no aplicativo)
  envia ao codeDB, uma vez, o nome e o e-mail que você digitar; o `codedb-lens license renew` (ou **Renovar chave**) envia
  só a chave atual e a versão do Lens. Nada sobre seus bancos. Depois disso a chave é verificada
  offline. Veja [Comparação e chave grátis](compare.md#o-que-e-enviado-e-quando).
- **Atualizações assinadas:** o `codedb-lens update` verifica uma assinatura Ed25519 antes de trocar o binário.
- **Senhas** ficam apenas no cofre de credenciais do sistema, nunca em texto puro nem como argumento.
- **Relatórios** podem conter nomes de servidores e texto de queries. Trate-os como documentos internos.
