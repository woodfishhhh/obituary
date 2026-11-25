# Início rápido

O Obituary dá uma vida útil a arquivos e pastas. Quando ela termina, eles são enterrados em um cemitério; você pode ressuscitá-los a qualquer momento durante o período de retenção (7 dias por padrão), e eles só são realmente excluídos depois disso. Ele foi criado para agentes de IA: scripts de depuração, logs, capturas de tela e saídas de testes produzidos por um agente têm uma vida útil desde o momento em que são criados. Ninguém precisa se lembrar de limpar, e o `rm -rf` do agente não destrói mais nada para sempre.

## 1. Instalar

Windows 10 / 11, macOS e Linux (x64 e ARM64):

| | Windows | macOS | Linux |
|---|---|---|---|
| Instalar com | Scoop, o script `install.ps1` (winget em revisão) | Homebrew, o script `install.sh` | o script `install.sh` |
| Verificação de vencimentos | processo residente na bandeja, Agendador de Tarefas como alternativa | agente launchd do usuário | temporizador `systemd --user`; crontab do usuário sem systemd |
| CLI, cemitério e ressurreição, registro automático (opcional), guardião de exclusão, integração com agentes (hooks, Skill, MCP), contêineres Docker, políticas de pastas | sim | sim | sim |
| Bandeja, janela pequena, menu Iniciar e atalhos da área de trabalho | sim | use a CLI: `obit list`, `obit graveyard`, `obit resurrect` | como no macOS |
| Menu de contexto e datas de enterro no gerenciador de arquivos | Explorador de Arquivos | Finder: Ações Rápidas, Etiquetas de data | Nautilus: Scripts · Dolphin: menu, Etiquetas de data |
| Modo Lixeira | sim | apenas o cemitério | apenas o cemitério |

**O jeito mais fácil: deixe o agente fazer isso.** Envie esta frase para o agente que você usa (Claude Code, Codex, Cursor e outros). Ele instala o Obituary, conecta-o a si mesmo e para quando a decisão for sua.

```text
Read https://obituary.woodfish.site/agent-install.md and follow it to install Obituary and connect it to yourself.
```

**Manualmente:** escolha seu sistema operacional abaixo e execute `obit doctor` para verificar a instalação.

### macOS / Linux

No macOS, o Homebrew instala em uma linha:

```bash
brew install --cask woodfishhhh/tap/obituary
```

Ou baixe e leia o script de instalação e depois execute-o (macOS e Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh
bash install.sh
# Custom directory: --install-dir "$HOME/bin"; preview only: --dry-run
```

O script instala a versão mais recente por padrão (`--version <version>` escolhe outra). O arquivo é baixado em quatro partes ao mesmo tempo (algumas rotas limitam a velocidade de cada conexão) e volta para uma única conexão quando isso não funciona. Ele detecta o sistema operacional e a arquitetura, verifica o SHA-256 da mesma fonte de download e instala `obit` e `obitd` em `~/.local/bin`. Siga as instruções do PATH e execute `obit --version`. Use `--mirror cn` para o espelho da China continental; o script está em `https://cn.obituary.woodfish.site/install.sh`.

Na primeira utilização no macOS, execute `obit --version` e `obitd --help`. Se o Gatekeeper pedir aprovação, abra Ajustes do Sistema → Privacidade e Segurança, escolha “Abrir Mesmo Assim” para cada executável cuja origem você verificou e tente novamente.

No WSL, mantenha os projetos e a configuração do agente no sistema de arquivos nativo do Linux, por exemplo em `~/projects`. Montagens DrvFS como `/mnt/c` podem não oferecer a renomeação segura exigida pela instalação. Use apenas a edição para Windows para definir a vida útil, enterrar ou restaurar arquivos nessas unidades.

No Docker, se o overlayfs do contêiner não fornecer identificadores persistentes de arquivo, `obit mark` recusa o registro e deixa o arquivo no lugar. Para restaurar os arquivos após a expiração, coloque-os e o diretório de dados do Obituary em um volume ou uma montagem do tipo bind com um sistema de arquivos que forneça esses identificadores, como ext4.

### Windows

::: code-group

```powershell [Scoop]
scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket
scoop install obituary
```

```powershell [winget]
# In review, available once approved
winget install woodfishhhh.Obituary
```

```powershell [Install script]
# Install script: download it, read it, then run it
# (installs to %LOCALAPPDATA%\Programs\obituary, adds it to your user PATH, creates a Start menu shortcut)
irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1
.\install.ps1          # installs the latest release; -WhatIf previews, -NoPath leaves PATH alone
```

:::

- As três opções baixam o mesmo arquivo do release do GitHub e verificam o SHA-256 antes de descompactá-lo; uma divergência interrompe a instalação.
- Assinatura de código: a [SignPath.io](https://about.signpath.io/) fornece assinatura de código gratuita, e o certificado é da [SignPath Foundation](https://signpath.org/). Consulte a [política de assinatura de código](code-signing) e a [política de privacidade](privacy).
- O Obituary ainda não tem assinatura de código. Se você baixar o arquivo pelo navegador, o Windows pode mostrar “O Windows protegeu o computador”; escolha “Mais informações → Executar assim mesmo”. O Scoop, o winget e o script de instalação normalmente não exibem esse aviso.
- A janela pequena `obit-gui.exe` precisa do runtime WebView2 (integrado ao Windows 11 e presente na maioria dos computadores com Windows 10) e funciona apenas em x64; a versão ARM64 inclui a linha de comando e a bandeja.
Depois, verifique a instalação:

```powershell
obit doctor              # health check: short summary, full HTML report (--open, --verbose, --json)
```

## 2. Conectar seus agentes

```powershell
obit init -g                 # Claude Code, all projects; also the Skill (and desktop integration on Windows)
obit init -g --agent codex   # Codex
```

Reinicie o agente depois. Para outros agentes, o servidor MCP e o Agent Skill, consulte [Conecte os agentes](./agents).

No macOS / Linux, o Claude Code usa `~/.claude`, o Codex usa `~/.codex` e as Skills compartilhadas usam `~/.agents/skills`. As substituições `CLAUDE_CONFIG_DIR` e `CODEX_HOME` também são respeitadas.

Com a supervisão de desktop ativa, a bandeja trata os vencimentos e uma tarefa independente verifica pendências a cada hora. Sem supervisão da bandeja para esse diretório de dados, a tarefa é executada a cada cinco minutos.

```bash
obit daemon enable --dry-run
obit daemon enable
obit doctor
```

O macOS usa o launchd do usuário conectado; o Linux usa `systemd --user`. No Linux sem systemd, agende `obitd --once` a cada 5 minutos no crontab do usuário, fornecendo os caminhos absolutos para `obitd`, o diretório de dados `--home` e o diretório de configuração `--config-dir`. Os hooks do agente também iniciam uma verificação em segundo plano quando chega a hora do trabalho.

## 3. Comandos comuns

Substitua `<id>` pelo número de um recibo de `obit add` ou `obit bury`, ou pelo número mostrado em `obit list` / `obit graveyard`.

```powershell
obit add debug.log 2h        # bury it in 2 hours (alias: mark)
obit list                    # what is left and for how long
obit extend <id> 1d          # one more day
obit bury old-build          # bury right away
obit graveyard               # what is in the graveyard and when it will be purged
obit resurrect <id>          # put it back, never overwriting
obit resurrect --last        # undo the last deletion
```

Para o restante, consulte [Comandos](./commands).

## Próximos passos

| Para descobrir | Leia |
|---|---|
| Como funcionam o registro, o enterro, a ressurreição e a exclusão permanente, e onde fica o cemitério | [Como funciona](./how-it-works) |
| Conectar Claude Code, Codex e outros agentes | [Conecte os agentes](./agents) |
| O que entra: registros de agentes, menu de contexto, registro automático opcional | [O que entra](./auto-register) |
| Transformar o `rm -rf` de um agente em um enterro que pode ser desfeito | [Guardião de exclusão](./delete-guard) |
| O que acontece quando um agente acessa um arquivo enterrado | [Avisos do cemitério](./grave-notices) |
| Vidas úteis para pastas como Downloads e Área de Trabalho | [Políticas de pastas](./folder-policies) |
| Usar o Obituary sem a linha de comando | [Janela, bandeja e menu de contexto](./desktop) |
| Contêineres Docker iniciados em uma sessão | [Contêineres Docker](./docker) |
| Todos os comandos | [Comandos](./commands) |
| Remover o Obituary | [Desinstalar](./uninstall) |
| Qualquer outra coisa | [FAQ](./faq) |
