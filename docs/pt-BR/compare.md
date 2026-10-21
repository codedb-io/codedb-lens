# Comparação e chave grátis

O `compare` mostra o que mudou entre duas execuções da mesma instância: achados novos, achados resolvidos, achados que
persistem e regras que passaram a rodar ou deixaram de rodar. Ele lê os relatórios JSON que o `check` já salva. Não se
conecta ao banco nem à internet.

## Obter a chave grátis

O `compare` precisa de uma chave grátis, válida por 90 dias. Renovar é grátis: basta se cadastrar de novo com o mesmo
e-mail.

- **Aplicativo:** clique em **Ativar chave grátis**, no rodapé da barra lateral.
- **Linha de comando:**

```bash
codedb-lens register                     # pergunta cada campo
codedb-lens register --name "Maria Silva" --email maria@empresa.com.br --accept-privacy
```

A chave também chega por e-mail. Num computador sem acesso à internet, ative a chave do e-mail com
`codedb-lens license import <chave>`, ou cole-a em **Já tem uma chave?** no aplicativo.

### O que é enviado, e quando

O cadastro é o **único** momento em que o Lens fala com o codeDB, e só acontece quando você pede a chave. O Lens envia
um pedido a `https://api.codedb.com.br/lens/register` com:

| Campo | Obrigatório |
|---|---|
| Nome, e-mail | sim |
| Empresa, cargo, quantidade de instâncias que você gerencia | não |
| Versão e idioma do Lens | sim (para enviar o e-mail no seu idioma) |
| Concordância com a Política de Privacidade | sim |
| Se você quer novidades e contato comercial | não, desmarcado por padrão |
| Versão do texto de consentimento exibido | sim |

Nada sobre seus bancos, servidores, relatórios ou uso é enviado. O contato comercial só acontece se você marcar essa
opção **e** confirmar pelo e-mail.

Do lado do codeDB, o IP é guardado truncado (/24 no IPv4, /48 no IPv6). O registro é apagado 30 dias depois do
vencimento da chave, ou depois de 24 meses sem novo cadastro se você confirmou o contato comercial. O link
**Apagar meus dados** do e-mail apaga o registro inteiro na hora. Veja a
[Política de Privacidade](https://app.codedb.com.br/privacidade).

### Como a chave funciona

A chave é assinada com Ed25519 e verificada **offline** com uma chave pública embutida no Lens. O Lens nunca chama o
codeDB para validá-la. Ela fica no cofre de credenciais do sistema (ou num arquivo legível só por você onde não há
cofre, como em servidores Linux). A chave não contém seu nome nem seu e-mail.

```bash
codedb-lens license            # validade, recursos e instância vinculada
codedb-lens license remove     # apaga a chave deste computador
```

## Comparar execuções

```bash
codedb-lens compare --profile prod-sql01           # as duas últimas execuções de um perfil
codedb-lens compare 2026-10-01_090000.json 2026-10-20_090000.json
```

No aplicativo, abra uma conexão e clique em **Comparar com o anterior**, em *Histórico*.

O resultado é um HTML autônomo (português/inglês) e um JSON, salvos ao lado do relatório mais novo como
`compare_<anterior>_vs_<novo>`.

| Seção | Significado |
|---|---|
| Achados novos | Estava OK, *Sem dados* ou ausente; agora é achado |
| Resolvidos | Era achado; agora está OK |
| Persistem | Achado nas duas execuções, com a quantidade de linhas de evidência antes e depois |
| Mudanças de cobertura | A regra passou a rodar ou deixou de rodar (permissão, versão, catálogo). **Nunca conta como resolvida** |

Nota e cobertura aparecem sempre lado a lado. O Lens avisa quando a cobertura mudou mais de 5 pontos, quando as
execuções usaram versões diferentes do Lens ou quando o catálogo de regras é diferente, porque aí a variação da nota
pode não refletir o servidor.

## Uma instância por chave

A chave grátis compara uma instância (engine + nome do servidor). O primeiro `compare` faz o vínculo. Para mudar de
instância, rode `codedb-lens license reset` (1 vez a cada 30 dias). Para acompanhar todas as instâncias continuamente,
conheça o [codeDB](https://codedb.com.br/?utm_source=lens&utm_medium=docs&utm_campaign=compare).
