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

## Integração e execução em segundo plano

```powershell
obit init                       # Claude Code hooks and Skill, context menu, Start menu and desktop shortcuts; the tray starts at login
obit init -g --agent codex      # Codex
obit init -g --guard-rm         # delete guard
obit init -g --mcp              # add the MCP server
obit init --no-context-menu --no-shortcuts --no-skill   # skip the context menu, shortcuts or Skill
obit init -g --show             # what is installed
obit daemon enable              # tray at login plus the fallback scheduled task
obit daemon status
obit daemon disable
```

## Consultas e verificações

```powershell
obit doctor              # read-only check (--json)
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

As tabelas abaixo são geradas pela saída de `obit --help` compilada a partir do código atual.

### `obit add`
Aliases: `add`, `mark`
Uso: `obit add <path>... <duration> \| add <path>... --at <time> [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--at` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--force` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |
| `--no-daemon` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--note` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

Exemplos:

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`
Uso: `obit approve <proposal id>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

### `obit bury`
Uso: `obit bury <path>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--force` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |
| `--note` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit cancel`
Aliases: `cancel`, `spare`
Uso: `obit cancel <id\|path>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

### `obit completion`
Uso: `obit completion [command]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit completion bash`
Uso: `obit completion bash`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--no-descriptions` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit completion fish`
Uso: `obit completion fish [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--no-descriptions` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit completion powershell`
Uso: `obit completion powershell [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--no-descriptions` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit completion zsh`
Uso: `obit completion zsh [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--no-descriptions` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit config`
Uso: `obit config [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--create` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--language` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit config telemetry`
Uso: `obit config telemetry on\|off [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit daemon`
Uso: `obit daemon [command]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit daemon disable`
Uso: `obit daemon disable [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--dry-run` | nenhum | `false` | Mostra apenas as ações previstas, sem alterar arquivos ou serviços. |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit daemon enable`
Uso: `obit daemon enable [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--dry-run` | nenhum | `false` | Mostra apenas as ações previstas, sem alterar arquivos ou serviços. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--resident` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit daemon start`
Uso: `obit daemon start [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit daemon status`
Uso: `obit daemon status [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

### `obit daemon stop`
Uso: `obit daemon stop [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit docker`
Uso: `obit docker [command]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit docker add`
Uso: `obit docker add <container name\|ID> <lifespan> [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

### `obit docker list`
Uso: `obit docker list [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

### `obit doctor`
Uso: `obit doctor [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |
| `--verbose` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit extend`
Uso: `obit extend <id\|path>... <duration> [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

Exemplos:

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`
Uso: `obit git-clean [options] [--] [pathspec]... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit graveyard`
Aliases: `graveyard`, `graves`
Uso: `obit graveyard [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--here` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

### `obit help`
Uso: `obit help [command] [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit hook`
Uso: `obit hook [command]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit hook antigravity`
Uso: `obit hook antigravity [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook audit`
Uso: `obit hook audit [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--lines` | `-n` | `40` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook claude`
Uso: `obit hook claude [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook cline`
Uso: `obit hook cline [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook codebuddy`
Uso: `obit hook codebuddy [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook codebuddy-ide`
Uso: `obit hook codebuddy-ide [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook codex`
Uso: `obit hook codex [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook copilot`
Uso: `obit hook copilot [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook cursor`
Uso: `obit hook cursor [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook droid`
Uso: `obit hook droid [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook gemini`
Uso: `obit hook gemini [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook hermes`
Uso: `obit hook hermes [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook kilocode`
Uso: `obit hook kilocode [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook kimi`
Uso: `obit hook kimi [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook lingma`
Uso: `obit hook lingma [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook minimax-code`
Uso: `obit hook minimax-code [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook omp`
Uso: `obit hook omp [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook openclaw`
Uso: `obit hook openclaw [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook opencode`
Uso: `obit hook opencode [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook pi`
Uso: `obit hook pi [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook qoder`
Uso: `obit hook qoder [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook qoder-cn`
Uso: `obit hook qoder-cn [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook trae`
Uso: `obit hook trae [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook trae-cn`
Uso: `obit hook trae-cn [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook vibe`
Uso: `obit hook vibe [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit hook zcode`
Uso: `obit hook zcode [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--event` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--notice-only` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit init`
Uso: `obit init [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--agent` | nenhum | `[claude]` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--context-menu` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--dry-run` | nenhum | `false` | Mostra apenas as ações previstas, sem alterar arquivos ou serviços. |
| `--format` | nenhum | `"text"` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--global` | `-g` | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--guard-rm` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--mcp` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--no-context-menu` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--no-shortcuts` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--no-skill` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--show` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--skill` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--uninstall` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--yes` | nenhum | `false` | Confirma a operação sem perguntar novamente. |

Exemplos:

```text
obit init -g                 # Claude Code, for every project (~/.claude)
obit init                    # Claude Code, this project only (.claude/)
obit init -g --agent codex   # Codex (~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall     # remove only what Obituary added
obit init -g --show          # what is installed
obit init --show --format markdown > docs/agents.md
```

### `obit list`
Aliases: `list`, `ls`
Uso: `obit list [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

### `obit log`
Uso: `obit log [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |
| `--lines` | `-n` | `20` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit mcp`
Uso: `obit mcp [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit mktemp`
Uso: `obit mktemp <lifespan> [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |
| `--name` | nenhum | `"tmp"` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--session` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

Exemplos:

```text
obit mktemp 2h --name screenshots
```

### `obit proposals`
Uso: `obit proposals [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

### `obit purge`
Aliases: `purge`, `cremate`
Uso: `obit purge [id\|original path]... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--all` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--dry-run` | nenhum | `false` | Mostra apenas as ações previstas, sem alterar arquivos ou serviços. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |
| `--yes` | nenhum | `false` | Confirma a operação sem perguntar novamente. |

### `obit reject`
Uso: `obit reject <proposal id>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

### `obit remove-item`
Uso: `obit remove-item [-Recurse] [-Force] [-Path\|-LiteralPath] <path> [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit resurrect`
Aliases: `resurrect`, `raise`
Uso: `obit resurrect <id\|original path>... \| --last [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |
| `--last` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit rm`
Uso: `obit rm [options] <path>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit rmdir`
Uso: `obit rmdir [options] <path>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit rules`
Uso: `obit rules [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

### `obit rules apply`
Uso: `obit rules apply <path>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |
| `--session` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit rules check`
Uso: `obit rules check <path>... [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |
| `--session` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit stats`
Aliases: `stats`, `eulogy`
Uso: `obit stats [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

### `obit sweep`
Aliases: `sweep`, `reap`
Uso: `obit sweep [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--dry-run` | nenhum | `false` | Mostra apenas as ações previstas, sem alterar arquivos ou serviços. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |

### `obit trust`
Uso: `obit trust [rules file] [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--yes` | nenhum | `false` | Confirma a operação sem perguntar novamente. |

### `obit uninstall`
Uso: `obit uninstall [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--dry-run` | nenhum | `false` | Mostra apenas as ações previstas, sem alterar arquivos ou serviços. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--purge-data` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--yes` | nenhum | `false` | Confirma a operação sem perguntar novamente. |

### `obit untrust`
Uso: `obit untrust [rules file] [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Mostra a ajuda. |

### `obit update`
Uso: `obit update [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--apply` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--check` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--relaunch-gui` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--wait-pid` | nenhum | `0` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

### `obit watch`
Uso: `obit watch <directory> <lifespan> \| watch --list \| watch --remove <directory> [flags]`

| Parâmetro | Curto | Padrão | Descrição |
| --- | --- | --- | --- |
| `--age` | nenhum | `"first-seen"` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--clear-exclusion` | nenhum | `0` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--help` | `-h` | `false` | Mostra a ajuda. |
| `--json` | nenhum | `false` | Exibe a saída em JSON. |
| `--list` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--no-daemon` | nenhum | `false` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--policy` | nenhum | `0` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--remove` | nenhum | `""` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |
| `--remove-id` | nenhum | `0` | O comportamento atual deste parâmetro é gerado pelo código; consulte também a ajuda. |

<!-- generated-command-reference:end -->
