# Comandos

Substitua `<id>` pelo número informado por `obit add` ou `obit bury`, ou consulte a lista de `obit list` ou `obit graveyard`. As unidades de duração `s`, `m`, `h`, `d` e `w` podem ser combinadas, por exemplo `1d12h`.

## Vida útil

```powershell
obit add debug.log 2h                       # bury it in 2 hours (alias: mark)
obit add output\shots 1d12h                 # folders work too
obit add report.zip --at "2026-12-01 18:00" # at a given moment
obit list                                   # what is left and for how long
obit extend <id> 1d                         # one more day
obit spare <id>                             # stop tracking, keep the file (alias: cancel)
obit reap                                   # bury everything that is due now (alias: sweep; --dry-run previews)
```

## O cemitério

```powershell
obit bury old-build                         # bury right away
obit graveyard                              # what is in the graveyard and when it will be purged (--here: this folder only)
obit resurrect <id>                         # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last                       # undo the last deletion: everything one obit bury / obit rm buried comes back
obit purge                                  # purge what is past retention (--all for everything, after confirming; --dry-run previews)
obit purge <id>                             # purge this grave now (permanent; asks first, --yes outside a terminal)
```

Em `resurrect` e `purge`, um número sem caminho (`5` ou `#5`) é sempre um ID. Se o ID não existir, o comando retorna um erro; nunca interpreta o número como nome de arquivo. Para um arquivo de nome numérico, use `./5` ou um caminho absoluto.

## Comandos de exclusão recuperável

```powershell
obit rm -rf build            # rm's common options; every path is buried
obit rmdir empty-dir
obit git-clean -fdx
obit remove-item .\build     # one literal path only
```

## Regras e propostas

```powershell
obit rules                       # the rules in effect here
obit rules check debug.log out/  # preview what would be registered and by which rule
obit rules apply debug.log       # register by rule
obit trust                       # trust this project's .obitrules (--yes when non-interactive)
obit proposals                   # proposals waiting for you, and what was registered without asking
obit approve 3                   # approve
obit reject 3                    # reject
$dir = obit mktemp 1d            # a temporary folder that buries itself
```

## Políticas de pasta e Docker

```powershell
obit watch C:\Users\you\Downloads 2w
obit watch --list
obit watch --remove C:\Users\you\Downloads
obit docker add web-1 3d
obit docker list
```

Use `obit mark --docker-volume cache 3d` (ou `add`) para enterrar `cache` em três dias, ou `obit bury --docker-volume -- cache` para enterrá-lo agora. [Contêineres e volumes do Docker](./docker)

## Integração e execução em segundo plano

```powershell
obit init                       # Claude Code hooks and Skill, context menu, Start menu and desktop shortcuts; the tray starts at login
obit init -g --agent codex      # Codex
obit init -g --guard-rm         # delete guard
obit init -g --mcp              # add the MCP server
obit init --no-context-menu --no-shortcuts --no-skill   # skip the context menu, shortcuts or Skill
obit init -g --uninstall --no-context-menu   # on uninstall keep the context menu and remove the rest; --no-shortcuts and --no-skill work the same way
obit init -g --show             # what is installed
obit daemon enable              # tray at login plus the fallback scheduled task
obit daemon status
obit daemon disable
```

Com a supervisão de desktop ativa, a bandeja trata os vencimentos e uma tarefa independente verifica pendências a cada hora. Sem supervisão da bandeja para esse diretório de dados, a tarefa é executada a cada cinco minutos.

## Consultas e verificações

```powershell
obit doctor              # health check: short summary, full HTML report (--open, --verbose, --json)
obit stats               # eulogy: how much was buried, space freed, resurrections (alias: eulogy)
obit log                 # the full record of everything done
obit config              # show the configuration; --create writes a commented default
obit config telemetry off # disable usage reporting and clear the local installation ID; update checks are separate
obit hook audit          # with OBIT_HOOK_AUDIT=1: what each hook received and did
obit uninstall           # remove everything Obituary set up (try --dry-run first)
```

Os comandos que retornam informações ou resultados (`list`, `graveyard`, `proposals`, `stats`, `mark`, `bury`, `resurrect` e outros) aceitam `--json` para scripts e agentes. Os que encapsulam comandos de exclusão, como `obit rm`, mantêm o formato de saída original e não aceitam `--json`.

## Variáveis de ambiente

| Variável | Efeito |
|---|---|
| `OBIT_DISABLED=1` | Desativa os hooks temporariamente |
| `OBIT_TELEMETRY=0`, `DO_NOT_TRACK=1` | Desativa o envio de estatísticas de uso neste processo; em CI não há envio |
| `OBIT_GUARD_RM=0` | Desativa temporariamente o guardião de exclusão |
| `OBIT_HOOK_AUDIT=1` | Registra o que cada hook recebeu e fez |
| `OBIT_TRUST_PROJECT_RULES=1` | Confia no arquivo `.obitrules` do projeto em CI |
| `OBIT_HOME`, `OBIT_CONFIG_DIR`, `OBIT_USER_HOME` | Alteram os locais dos diretórios de dados, de configuração e da pasta pessoal do usuário |



## Parâmetros compatíveis dos wrappers

- git-clean aceita as opções do Git clean -d, -f, -n, -q, -x, -X, -e pattern, --exclude pattern e --, além de --dry-run, --force e --quiet.
- rm aceita -r, -R, -f, -d, -v, --recursive, --force, --dir, --verbose, --preserve-root e --, seguidos pelos caminhos.
- rmdir aceita apenas -v ou --verbose e caminhos; rmdir -p é rejeitado.
- No Windows, remove-item aceita somente -Recurse, -Force, -Path ou -LiteralPath e exatamente um caminho literal. Para curingas no Bash, use obit rm.

<!-- generated-command-reference:start -->
## Referência de parâmetros

### `obit`

Obituary (讣告), a lixeira dos agentes de IA: dá uma vida útil aos arquivos e, quando ela se esgota, eles são enterrados em um cemitério de onde podem ser ressuscitados

Uso: `obit [command]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de obit |
| `--version` | `-v` | `false` | Versão do obit |

### `obit add`

Dar uma vida útil a caminhos (registrar de novo um caminho acompanhado reinicia o vencimento)

Aliases: `add`, `mark`
Uso: `obit add <path>... <duration> | add <path>... --at <time> [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--at` | nenhum | `""` | Expirar em um horário local: "2006-01-02 15:04", "2006-01-02" ou "15:04" |
| `--docker-volume` | nenhum | `false` | Interpretar os argumentos como nomes de volumes do Docker |
| `--force` | nenhum | `false` | Permitir pastas que contêm um repositório .git |
| `--help` | `-h` | `false` | Ajuda de add |
| `--json` | nenhum | `false` | Exibir JSON |
| `--no-daemon` | nenhum | `false` | Não iniciar o daemon em segundo plano |
| `--note` | nenhum | `""` | Nota de texto livre exibida na lista |

Exemplos:

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`

Aprovar propostas: registrar o que elas listam

Uso: `obit approve <proposal id>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de approve |
| `--json` | nenhum | `false` | Exibir JSON |

### `obit bury`

Mover caminhos para o cemitério agora (restaure com obit resurrect)

Uso: `obit bury <path>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--docker-volume` | nenhum | `false` | Interpretar os argumentos como nomes de volumes do Docker |
| `--force` | nenhum | `false` | Permitir pastas que contêm um repositório .git |
| `--help` | `-h` | `false` | Ajuda de bury |
| `--json` | nenhum | `false` | Exibir JSON |
| `--note` | nenhum | `""` | Nota de texto livre guardada junto com o enterro |

### `obit cancel`

Poupar caminhos ou IDs de contêiner: parar de acompanhá-los e manter os objetos

Aliases: `cancel`, `spare`
Uso: `obit cancel <id|path>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de cancel |
| `--json` | nenhum | `false` | Exibir JSON |

### `obit completion`

Gerar o script de autocompletar do obit para o shell indicado.

Uso: `obit completion [command]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de completion |

### `obit completion bash`

Gerar o script de autocompletar para o shell bash.

Uso: `obit completion bash`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de bash |
| `--no-descriptions` | nenhum | `false` | Desativar as descrições do autocompletar |

### `obit completion fish`

Gerar o script de autocompletar para o shell fish.

Uso: `obit completion fish [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de fish |
| `--no-descriptions` | nenhum | `false` | Desativar as descrições do autocompletar |

### `obit completion powershell`

Gerar o script de autocompletar para o powershell.

Uso: `obit completion powershell [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de powershell |
| `--no-descriptions` | nenhum | `false` | Desativar as descrições do autocompletar |

### `obit completion zsh`

Gerar o script de autocompletar para o shell zsh.

Uso: `obit completion zsh [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de zsh |
| `--no-descriptions` | nenhum | `false` | Desativar as descrições do autocompletar |

### `obit config`

Exibir o arquivo de configuração e as definições em vigor (--create grava os valores padrão)

Uso: `obit config [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--create` | nenhum | `false` | Gravar um arquivo de configuração com os valores padrão |
| `--help` | `-h` | `false` | Ajuda de config |
| `--language` | nenhum | `""` | Definir o idioma: auto, en ou zh-CN |

### `obit config telemetry`

Estatísticas de uso: um ID de instalação aleatório, versão, sistema operacional, arquitetura, idioma da interface e canal de instalação ajudam a medir o número de instalações e o uso das versões. Nenhum arquivo, caminho ou comando é enviado; endereços IP e user agents não são armazenados. Desative com obit config telemetry off ou OBIT_TELEMETRY=0.

Uso: `obit config telemetry on|off [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de telemetry |

### `obit daemon`

Controlar o daemon em segundo plano (obitd)

Uso: `obit daemon [command]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de daemon |

### `obit daemon disable`

Remover a tarefa agendada e o daemon residente

Uso: `obit daemon disable [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--dry-run` | nenhum | `false` | Apenas exibir as alterações |
| `--help` | `-h` | `false` | Ajuda de disable |

### `obit daemon enable`

Ativar a tarefa do usuário (a cada hora com supervisão da bandeja; caso contrário, a cada 5 minutos) e, no Windows, a bandeja ao entrar

Uso: `obit daemon enable [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--dry-run` | nenhum | `false` | Apenas exibir as alterações |
| `--help` | `-h` | `false` | Ajuda de enable |
| `--resident` | nenhum | `false` | Ativar a bandeja sem alterar a tarefa agendada de reserva |

### `obit daemon start`

Iniciar o daemon agora

Uso: `obit daemon start [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de start |

### `obit daemon status`

Exibir o estado do daemon e o próximo vencimento

Uso: `obit daemon status [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de status |
| `--json` | nenhum | `false` | Exibir JSON |

### `obit daemon stop`

Parar o daemon

Uso: `obit daemon stop [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de stop |

### `obit docker`

Acompanhar contêineres Docker pelo ID completo imutável e pela data de criação. No vencimento, o contêiner é parado e sua camada gravável é mantida. A ressurreição o inicia. Depois de [graveyard] retention_days, a exclusão permanente remove contêineres parados sem remover volumes; isso vale também no modo de lixeira. Imagens, volumes e redes são preservados.

Uso: `obit docker [command]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de docker |

### `obit docker add`

Registrar um contêiner Docker; no vencimento ele é parado

Uso: `obit docker add <container name|ID> <lifespan> [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de add |
| `--json` | nenhum | `false` | Exibir JSON |

### `obit docker list`

Listar os contêineres Docker registrados

Uso: `obit docker list [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de list |
| `--json` | nenhum | `false` | Exibir JSON |

### `obit doctor`

Verificar a instalação, as integrações e os dados; a única coisa que grava é o próprio relatório

Uso: `obit doctor [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de doctor |
| `--json` | nenhum | `false` | Exibir JSON |
| `--open` | nenhum | `false` | Abrir o relatório HTML completo no navegador padrão |
| `--verbose` | nenhum | `false` | Imprimir todas as verificações em texto completo, com os detalhes dos recursos dos agentes |

### `obit extend`

Prolongar: adiar os vencimentos (contados a partir de agora se já venceram)

Uso: `obit extend <id|path>... <duration> [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de extend |
| `--json` | nenhum | `false` | Exibir JSON |

Exemplos:

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`

Enterrar os caminhos selecionados pela simulação do git clean

Uso: `obit git-clean [options] [--] [pathspec]... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de git-clean |

### `obit graveyard`

Listar o que está no cemitério e ainda pode ser ressuscitado

Aliases: `graveyard`, `graves`
Uso: `obit graveyard [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de graveyard |
| `--here` | nenhum | `false` | Mostrar apenas caminhos originais no diretório atual e em seus subdiretórios |
| `--json` | nenhum | `false` | Exibir JSON |

### `obit help`

Exibir a ajuda de qualquer comando do aplicativo.

Uso: `obit help [command] [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de help |

### `obit hook`

Pontos de entrada dos hooks de agente (chamados pelo agente, não por pessoas)

Uso: `obit hook [command]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de hook |

### `obit hook antigravity`

Processar um evento de hook do antigravity a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook antigravity [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de antigravity |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook audit`

Exibir as trilhas de auditoria dos hooks e do guardião de exclusão

Uso: `obit hook audit [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de audit |
| `--lines` | `-n` | `40` | Número de linhas por origem |

### `obit hook claude`

Processar um evento de hook do claude a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook claude [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de claude |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook cline`

Processar um evento de hook do cline a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook cline [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de cline |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook codebuddy`

Processar um evento de hook do codebuddy a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook codebuddy [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de codebuddy |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook codebuddy-ide`

Processar um evento de hook do codebuddy-ide a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook codebuddy-ide [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de codebuddy-ide |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook codex`

Processar um evento de hook do codex a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook codex [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de codex |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook copilot`

Processar um evento de hook do copilot a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook copilot [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de copilot |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook cursor`

Processar um evento de hook do cursor a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook cursor [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de cursor |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook deepseek-harness`

Processar um evento de hook do deepseek-harness a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook deepseek-harness [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de deepseek-harness |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook droid`

Processar um evento de hook do droid a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook droid [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de droid |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook gemini`

Processar um evento de hook do gemini a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook gemini [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de gemini |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook hermes`

Processar um evento de hook do hermes a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook hermes [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de hermes |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook kilocode`

Processar um evento de hook do kilocode a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook kilocode [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de kilocode |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook kimi`

Processar um evento de hook do kimi a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook kimi [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de kimi |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook lingma`

Processar um evento de hook do lingma a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook lingma [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de lingma |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook minimax-code`

Processar um evento de hook do minimax-code a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook minimax-code [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de minimax-code |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook omp`

Processar um evento de hook do omp a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook omp [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de omp |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook openclaw`

Processar um evento de hook do openclaw a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook openclaw [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de openclaw |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook opencode`

Processar um evento de hook do opencode a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook opencode [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de opencode |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook pi`

Processar um evento de hook do pi a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook pi [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de pi |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook qoder`

Processar um evento de hook do qoder a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook qoder [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de qoder |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook qoder-cn`

Processar um evento de hook do qoder-cn a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook qoder-cn [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de qoder-cn |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook trae`

Processar um evento de hook do trae a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook trae [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de trae |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook trae-cn`

Processar um evento de hook do trae-cn a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook trae-cn [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de trae-cn |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook vibe`

Processar um evento de hook do vibe a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook vibe [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de vibe |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook zcode`

Processar um evento de hook do zcode a partir de stdin; em caso de erro, a ação é permitida

Uso: `obit hook zcode [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | Nome do evento do host quando ele não vem no payload |
| `--help` | `-h` | `false` | Ajuda de zcode |
| `--notice-only` | nenhum | `false` | Entregar contexto informativo sem o guardião de exclusão |

### `obit hook finish`

Calcular os arquivos que uma sessão deixou para trás (iniciado em segundo plano pelos hooks)

Uso: `obit hook finish [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de finish |
| `--project` | nenhum | `""` | Raiz do projeto |
| `--session` | nenhum | `""` | ID da sessão do agente |

### `obit init`

Instalar (ou desinstalar com --uninstall) os hooks e as instruções do Obituary para um agente

Uso: `obit init [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--agent` | nenhum | `[claude]` | Agentes a integrar (repetir a opção ou separar por vírgulas; padrão: claude) |
| `--context-menu` | nenhum | `false` | Instalar os menus de contexto clássicos do Explorador de Arquivos (ativado por padrão) |
| `--dry-run` | nenhum | `false` | Apenas exibir as alterações |
| `--format` | nenhum | `"text"` | Formato de saída de --show: text ou markdown (cobertura dos agentes) |
| `--global` | `-g` | `false` | Para todo o usuário, em vez de só este projeto |
| `--guard-rm` | nenhum | `false` | Negar os comandos de exclusão compatíveis e sugerir alternativas do obit que podem ser restauradas |
| `--help` | `-h` | `false` | Ajuda de init |
| `--mcp` | nenhum | `false` | Registrar o servidor MCP stdio do Obituary |
| `--no-context-menu` | nenhum | `false` | Pular os menus de contexto do Explorador de Arquivos |
| `--no-shortcuts` | nenhum | `false` | Pular os atalhos do menu Iniciar e da área de trabalho |
| `--no-skill` | nenhum | `false` | Deixar o Agent Skill do Obituary inalterado |
| `--show` | nenhum | `false` | Exibir o que está instalado |
| `--skill` | nenhum | `false` | Instalar o Agent Skill do Obituary (ativado por padrão) |
| `--uninstall` | nenhum | `false` | Remover o que o Obituary adicionou |
| `--verbose` | nenhum | `false` | Com --show: listar os detalhes dos recursos de cada agente |
| `--yes` | nenhum | `false` | Aplicar sem perguntar |

Exemplos:

```text
obit init -g  # Claude Code, para todos os projetos (~/.claude)
obit init  # Claude Code, somente este projeto (.claude/)
obit init -g --agent codex  # Codex (~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall  # Remover apenas o que o Obituary adicionou
obit init -g --show  # O que está instalado
obit init --show --format markdown > docs/agents.md
```

### `obit list`

Exibir os caminhos acompanhados e os contêineres Docker, do vencimento mais próximo ao mais distante

Aliases: `list`, `ls`
Uso: `obit list [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de list |
| `--json` | nenhum | `false` | Exibir JSON |

### `obit log`

Exibir o log de auditoria de tudo o que o Obituary fez

Uso: `obit log [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de log |
| `--json` | nenhum | `false` | Exibir JSON |
| `--lines` | `-n` | `20` | Número de eventos |

### `obit mcp`

Servir as ferramentas do Obituary por MCP stdio

Uso: `obit mcp [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de mcp |

### `obit mktemp`

Criar um diretório temporário que se enterra sozinho e exibir seu caminho

Uso: `obit mktemp <lifespan> [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de mktemp |
| `--json` | nenhum | `false` | Exibir JSON |
| `--name` | nenhum | `"tmp"` | Prefixo do nome do diretório |
| `--session` | nenhum | `""` | Sessão de agente a que o diretório pertence |

Exemplos:

```text
dir=$(obit mktemp 1d)
obit mktemp 2h --name screenshots
```

### `obit proposals`

Registros aguardando a sua aprovação e os feitos sem perguntar

Uso: `obit proposals [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de proposals |
| `--json` | nenhum | `false` | Exibir JSON |

### `obit purge`

Excluir enterros de arquivos selecionados ou com a retenção vencida. --all e referências explícitas ignoram a retenção, por isso pedem confirmação antes; fora de um terminal exigem --yes. Os contêineres Docker exigem que [graveyard] retention_days tenha passado, mesmo com --all ou com um ID explícito; contêineres em execução são tratados como ressuscitados. A remoção do Docker preserva os volumes e nunca força a remoção. O tratamento dos contêineres é idêntico no modo de lixeira.

Aliases: `purge`, `cremate`
Uso: `obit purge [id|original path]... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--all` | nenhum | `false` | Excluir permanentemente todos os enterros, não apenas os que passaram da retenção |
| `--dry-run` | nenhum | `false` | Mostrar apenas o que aconteceria |
| `--help` | `-h` | `false` | Ajuda de purge |
| `--json` | nenhum | `false` | Exibir JSON |
| `--yes` | nenhum | `false` | Excluir permanentemente --all ou os enterros selecionados sem perguntar (agentes: perguntar antes ao usuário) |

### `obit reject`

Rejeitar propostas

Uso: `obit reject <proposal id>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de reject |
| `--json` | nenhum | `false` | Exibir JSON |

### `obit remove-item`

Enterrar itens do sistema de arquivos usando um subconjunto limitado de parâmetros do PowerShell.

Uso: `obit remove-item [-Recurse] [-Force] [-Path|-LiteralPath] <path> [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de remove-item |

### `obit resurrect`

Restaurar arquivos sem sobrescrever; iniciar por ID os contêineres Docker enterrados

Aliases: `resurrect`, `raise`
Uso: `obit resurrect <id|original path>... | --last [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de resurrect |
| `--json` | nenhum | `false` | Exibir JSON |
| `--last` | nenhum | `false` | Restaurar o último lote de exclusões da sessão atual ou do projeto Git |

### `obit rm`

Mover caminhos para o cemitério agora (restaure com obit resurrect)

Uso: `obit rm [options] <path>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de rm |

### `obit rmdir`

Mover caminhos para o cemitério agora (restaure com obit resurrect)

Uso: `obit rmdir [options] <path>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de rmdir |

### `obit rules`

Mostrar as regras em vigor aqui: as integradas mais o .obitrules do projeto, depois que ele for marcado como confiável

Uso: `obit rules [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de rules |
| `--json` | nenhum | `false` | Exibir JSON |

### `obit rules apply`

Registrar caminhos por regra (arquivos rastreados pelo Git nunca são registrados)

Uso: `obit rules apply <path>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de apply |
| `--json` | nenhum | `false` | Exibir JSON |
| `--session` | nenhum | `""` | Sessão de agente de onde vieram os caminhos |

### `obit rules check`

Mostrar o que as regras registrariam para os caminhos (não altera nada)

Uso: `obit rules check <path>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de check |
| `--json` | nenhum | `false` | Exibir JSON |
| `--session` | nenhum | `""` | Sessão de agente de onde vieram os caminhos |

### `obit stats`

O que o Obituary fez: itens acompanhados, enterros, espaço recuperado, ressurreições

Aliases: `stats`, `eulogy`
Uso: `obit stats [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de stats |
| `--json` | nenhum | `false` | Exibir JSON |

### `obit sweep`

Colher agora: enterrar tudo o que venceu, sem esperar o processo em segundo plano

Aliases: `sweep`, `reap`
Uso: `obit sweep [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--dry-run` | nenhum | `false` | Mostrar apenas o que aconteceria |
| `--help` | `-h` | `false` | Ajuda de sweep |
| `--json` | nenhum | `false` | Exibir JSON |

### `obit trust`

Revisar e confiar no .obitrules do projeto (edições exigem uma nova revisão)

Uso: `obit trust [rules file] [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de trust |
| `--yes` | nenhum | `false` | Confiar sem perguntar (somente depois de revisar as regras) |

### `obit uninstall`

Remover as integrações e os serviços em segundo plano do Obituary; os dados são mantidos por padrão

Uso: `obit uninstall [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--dry-run` | nenhum | `false` | Apenas exibir o plano de desinstalação; não alterar arquivos nem serviços |
| `--help` | `-h` | `false` | Ajuda de uninstall |
| `--purge-data` | nenhum | `false` | Excluir permanentemente os túmulos listados e remover os dados pertencentes ao Obituary; manter os arquivos desconhecidos |
| `--yes` | nenhum | `false` | Confirmar o plano (e a exclusão dos dados somente se --purge-data também for informado) |

### `obit untrust`

Deixar de aplicar o .obitrules do projeto

Uso: `obit untrust [rules file] [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Ajuda de untrust |

### `obit update`

Verificar ou instalar uma atualização estável

Uso: `obit update [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--apply` | nenhum | `false` | Baixar, verificar e instalar a versão estável mais recente que está em cache (uma instalação manual substitui apenas os programas do próprio Obituary) |
| `--check` | nenhum | `false` | Verificar o GitHub agora, inclusive quando as verificações automáticas estão desativadas |
| `--help` | `-h` | `false` | Ajuda de update |
| `--relaunch-gui` | nenhum | `false` | Reabrir a janela depois de atualizar (Windows) |
| `--wait-pid` | nenhum | `0` | Esperar este processo terminar antes de atualizar (Windows) |

### `obit watch`

Dar a cada item de nível superior de uma pasta a sua própria vida útil

Uso: `obit watch <directory> <lifespan> | watch --list | watch --remove <directory> [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--age` | nenhum | `"first-seen"` | Base da idade: first-seen ou modified (requer aprovação) |
| `--clear-exclusion` | nenhum | `0` | Remover explicitamente a exceção com este ID (com --policy) |
| `--help` | `-h` | `false` | Ajuda de watch |
| `--json` | nenhum | `false` | Exibir JSON |
| `--list` | nenhum | `false` | Listar as políticas de pasta |
| `--no-daemon` | nenhum | `false` | Não iniciar o processo em segundo plano |
| `--policy` | nenhum | `0` | ID da política cuja exceção deve ser removida |
| `--remove` | nenhum | `""` | Remover uma política e cancelar as vidas úteis que ela registrou |
| `--remove-id` | nenhum | `0` | Remover uma política de pasta por ID, inclusive para caminhos ausentes |

<!-- generated-command-reference:end -->
