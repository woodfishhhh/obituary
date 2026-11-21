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
<!-- generated-command-reference:end -->
