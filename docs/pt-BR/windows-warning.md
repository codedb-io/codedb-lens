# Por que o Windows mostra um aviso

Ao executar o instalador do codeDB Lens ou o `codedb-lens.exe` pela primeira vez, o Windows pode mostrar **"O Windows
protegeu o computador"** (SmartScreen) ou dizer que o editor é desconhecido. Esta página explica o motivo, o que é seguro
fazer e como conferir que o arquivo é o que nós geramos.

## Por que acontece

O Lens é de código aberto e seus binários do Windows **ainda não têm assinatura de código**. Um certificado de assinatura
é um produto pago (recorrente e precificado para empresas), e os programas gratuitos para projetos de código aberto
exigem um projeto público com algum histórico, que o Lens ainda não tinha quando pediu. Preferimos dizer isso com
clareza a esconder.

Mesmo um arquivo assinado recebe o aviso do SmartScreen até construir "reputação" com muitos downloads. Arquivos sem
assinatura começam sem nenhuma. O aviso quer dizer *o Windows ainda não conhece este arquivo*. Não quer dizer que o
arquivo foi considerado malicioso.

## O que é seguro fazer

Na tela do SmartScreen, clique em **Mais informações** e depois em **Executar assim mesmo**. Faça isso só depois de
conferir o arquivo, como descrito abaixo, e só para um arquivo baixado da página de
[Releases no GitHub](https://github.com/codedb-io/codedb-lens/releases/latest) deste projeto.

## Verifique o download

Toda release publica o `checksums.txt` com o SHA-256 de cada arquivo.

1. Baixe o `checksums.txt` da mesma release.
2. Calcule o hash do seu arquivo no terminal:

    ```bat
    certutil -hashfile codedb-lens_windows_amd64_setup.exe SHA256
    ```

3. Compare com a linha desse arquivo no `checksums.txt`. Os valores têm que ser idênticos.

Duas verificações mais fortes:

- **Procedência do build (a partir da release seguinte à v0.6.0).** Cada arquivo tem uma atestação de build do GitHub que
  liga o arquivo a este repositório e à execução do workflow que o gerou. Com o [GitHub CLI](https://cli.github.com/):

    ```bash
    gh attestation verify codedb-lens_windows_amd64_setup.exe --repo codedb-io/codedb-lens
    ```

- **Checksums assinados.** O `checksums.txt.sig` é uma assinatura Ed25519 do `checksums.txt`. Você não precisa conferir à
  mão: o `codedb-lens update` verifica automaticamente, com a chave pública embutida no programa, antes de substituir
  qualquer coisa.

## Alternativas sem o aviso

- **Scoop:** `scoop bucket add codedb https://github.com/codedb-io/codedb-lens` e depois `scoop install codedb/codedb-lens`.
  O Scoop confere o hash no download.
- **Docker:** `docker run --rm -e PG_DSN -v "$PWD:/out" ghcr.io/codedb-io/codedb-lens check --engine pg --dsn-env PG_DSN --out /out`.
- **Pacotes Linux:** `.deb`, `.rpm` ou o `.tar.gz` da página de releases, se você puder rodar o Lens num host Linux ou no WSL.
- **Compilar do código-fonte:** com o [Go](https://go.dev/dl/) instalado, `git clone https://github.com/codedb-io/codedb-lens`
  e `go build ./cmd/codedb-lens` na pasta do repositório geram um binário compilado por você. A linha de comando é tudo o
  que o `check` precisa; o app de janela usa Wails (veja o repositório para os passos de build).

## Ambientes corporativos

Alguns computadores gerenciados bloqueiam programas sem assinatura de vez, sem o botão "Executar assim mesmo":
**AppLocker**, **Windows Defender Application Control (WDAC)** ou **Controle Inteligente de Aplicativos** (Smart App
Control). Nesse caso:

- rode o Lens pelo Docker, ou por um host Linux/WSL que tenha acesso ao banco; ou
- peça ao time de TI para liberar o arquivo **por hash**: passe o SHA-256 do `checksums.txt` da versão desejada. Liberar por
  hash não exige certificado.

## Transparência

- Código-fonte: [github.com/codedb-io/codedb-lens](https://github.com/codedb-io/codedb-lens) (Apache 2.0).
- A release é gerada pelo GitHub Actions a partir de uma tag, não em uma máquina pessoal:
  [`.github/workflows/release.yml`](https://github.com/codedb-io/codedb-lens/blob/main/.github/workflows/release.yml).
- O que o Lens envia pela rede está em [Privacidade](privacy.md): sem telemetria, e o banco só é lido, nunca alterado.

Se encontrar algo suspeito, veja o [SECURITY.md](https://github.com/codedb-io/codedb-lens/blob/main/SECURITY.md).
