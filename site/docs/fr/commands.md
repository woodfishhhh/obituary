# Commandes

Remplacez `<id>` par le numéro indiqué dans le résultat de `obit add` ou de `obit bury`, ou dans la liste de `obit list` ou de `obit graveyard`. Les unités de durée `s`, `m`, `h`, `d` et `w` se combinent, par exemple `1d12h`.

## Durée de vie

```powershell
obit add debug.log 2h                       # bury it in 2 hours (alias: mark)
obit add output\shots 1d12h                 # folders work too
obit add report.zip --at "2026-12-01 18:00" # at a given moment
obit list                                   # what is left and for how long
obit extend <id> 1d                         # one more day
obit spare <id>                             # stop tracking, keep the file (alias: cancel)
obit reap                                   # bury everything that is due now (alias: sweep; --dry-run previews)
```

## Le cimetière

```powershell
obit bury old-build                         # bury right away
obit graveyard                              # what is in the graveyard and when it will be purged (--here: this folder only)
obit resurrect <id>                         # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last                       # undo the last deletion: everything one obit bury / obit rm buried comes back
obit purge                                  # purge what is past retention (--all for everything, after confirming; --dry-run previews)
obit purge <id>                             # purge this grave now (permanent; asks first, --yes outside a terminal)
```

Pour `resurrect` et `purge`, un nombre sans chemin (`5` ou `#5`) désigne toujours un identifiant. Un identifiant inconnu produit une erreur : il n’est jamais interprété comme un nom de fichier. Si le fichier porte un nom numérique, indiquez `./5` ou son chemin absolu.

## Commandes de suppression récupérable

```powershell
obit rm -rf build            # rm's common options; every path is buried
obit rmdir empty-dir
obit git-clean -fdx
obit remove-item .\build     # one literal path only
```

## Règles et propositions

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

## Règles de dossier et Docker

```powershell
obit watch C:\Users\you\Downloads 2w
obit watch --list
obit watch --remove C:\Users\you\Downloads
obit docker add web-1 3d
obit docker list
```

## Intégration et tâches en arrière-plan

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

## Requêtes et vérifications

```powershell
obit doctor              # read-only check (--json)
obit stats               # eulogy: how much was buried, space freed, resurrections (alias: eulogy)
obit log                 # the full record of everything done
obit config              # show the configuration; --create writes a commented default
obit config telemetry off # disable usage reporting and clear the local installation ID; update checks are separate
obit hook audit          # with OBIT_HOOK_AUDIT=1: what each hook received and did
obit uninstall           # remove everything Obituary set up (try --dry-run first)
```

Les commandes de consultation ou de compte rendu (`list`, `graveyard`, `proposals`, `stats`, `mark`, `bury`, `resurrect`, entre autres) acceptent `--json` pour les scripts et les agents. Les commandes qui encapsulent une suppression, comme `obit rm`, conservent le format de sortie d’origine et n’acceptent pas `--json`.

## Variables d’environnement

| Variable | Effet |
|---|---|
| `OBIT_DISABLED=1` | Désactive temporairement les hooks |
| `OBIT_TELEMETRY=0`, `DO_NOT_TRACK=1` | Désactive l’envoi des statistiques d’utilisation pour ce processus ; la CI n’en envoie pas |
| `OBIT_GUARD_RM=0` | Désactive temporairement le garde de suppression |
| `OBIT_HOOK_AUDIT=1` | Consigne ce que chaque hook a reçu et ce qu’il a fait |
| `OBIT_TRUST_PROJECT_RULES=1` | Fait confiance au fichier `.obitrules` du projet en CI |
| `OBIT_HOME`, `OBIT_CONFIG_DIR`, `OBIT_USER_HOME` | Modifient respectivement l’emplacement des données, de la configuration et du répertoire personnel |



## Paramètres compatibles des wrappers

- git-clean accepte les options Git clean -d, -f, -n, -q, -x, -X, -e pattern, --exclude pattern et --, ainsi que --dry-run, --force et --quiet.
- rm accepte -r, -R, -f, -d, -v, --recursive, --force, --dir, --verbose, --preserve-root et --, suivis des chemins.
- rmdir n’accepte que -v ou --verbose et les chemins ; rmdir -p est refusé.
- Sous Windows, remove-item n’accepte que -Recurse, -Force, -Path ou -LiteralPath et un seul chemin littéral. Pour les jokers Bash, utilisez obit rm.

<!-- generated-command-reference:start -->
## Référence des paramètres

Les tableaux suivants sont générés à partir de la sortie de `obit --help` construite avec le code actuel.

### `obit add`
Alias: `add`, `mark`
Usage: `obit add <path>... <duration> \| add <path>... --at <time> [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--at` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--force` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |
| `--no-daemon` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--note` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

Exemples :

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`
Usage: `obit approve <proposal id>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

### `obit bury`
Usage: `obit bury <path>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--force` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |
| `--note` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit cancel`
Alias: `cancel`, `spare`
Usage: `obit cancel <id\|path>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

### `obit completion`
Usage: `obit completion [command]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit completion bash`
Usage: `obit completion bash`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--no-descriptions` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit completion fish`
Usage: `obit completion fish [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--no-descriptions` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit completion powershell`
Usage: `obit completion powershell [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--no-descriptions` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit completion zsh`
Usage: `obit completion zsh [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--no-descriptions` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit config`
Usage: `obit config [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--create` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--language` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit config telemetry`
Usage: `obit config telemetry on\|off [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit daemon`
Usage: `obit daemon [command]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit daemon disable`
Usage: `obit daemon disable [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--dry-run` | aucun | `false` | Affiche seulement les actions prévues, sans modifier les fichiers ni les services. |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit daemon enable`
Usage: `obit daemon enable [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--dry-run` | aucun | `false` | Affiche seulement les actions prévues, sans modifier les fichiers ni les services. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--resident` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit daemon start`
Usage: `obit daemon start [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit daemon status`
Usage: `obit daemon status [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

### `obit daemon stop`
Usage: `obit daemon stop [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit docker`
Usage: `obit docker [command]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit docker add`
Usage: `obit docker add <container name\|ID> <lifespan> [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

### `obit docker list`
Usage: `obit docker list [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

### `obit doctor`
Usage: `obit doctor [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |
| `--verbose` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit extend`
Usage: `obit extend <id\|path>... <duration> [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

Exemples :

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`
Usage: `obit git-clean [options] [--] [pathspec]... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit graveyard`
Alias: `graveyard`, `graves`
Usage: `obit graveyard [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--here` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

### `obit help`
Usage: `obit help [command] [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit hook`
Usage: `obit hook [command]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit hook antigravity`
Usage: `obit hook antigravity [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook audit`
Usage: `obit hook audit [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--lines` | `-n` | `40` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook claude`
Usage: `obit hook claude [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook cline`
Usage: `obit hook cline [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook codebuddy`
Usage: `obit hook codebuddy [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook codebuddy-ide`
Usage: `obit hook codebuddy-ide [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook codex`
Usage: `obit hook codex [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook copilot`
Usage: `obit hook copilot [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook cursor`
Usage: `obit hook cursor [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook droid`
Usage: `obit hook droid [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook gemini`
Usage: `obit hook gemini [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook hermes`
Usage: `obit hook hermes [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook kilocode`
Usage: `obit hook kilocode [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook kimi`
Usage: `obit hook kimi [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook lingma`
Usage: `obit hook lingma [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook minimax-code`
Usage: `obit hook minimax-code [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook omp`
Usage: `obit hook omp [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook openclaw`
Usage: `obit hook openclaw [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook opencode`
Usage: `obit hook opencode [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook pi`
Usage: `obit hook pi [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook qoder`
Usage: `obit hook qoder [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook qoder-cn`
Usage: `obit hook qoder-cn [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook trae`
Usage: `obit hook trae [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook trae-cn`
Usage: `obit hook trae-cn [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook vibe`
Usage: `obit hook vibe [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit hook zcode`
Usage: `obit hook zcode [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--notice-only` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit init`
Usage: `obit init [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--agent` | aucun | `[claude]` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--context-menu` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--dry-run` | aucun | `false` | Affiche seulement les actions prévues, sans modifier les fichiers ni les services. |
| `--format` | aucun | `"text"` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--global` | `-g` | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--guard-rm` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--mcp` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--no-context-menu` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--no-shortcuts` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--no-skill` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--show` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--skill` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--uninstall` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--yes` | aucun | `false` | Confirme l’opération sans demander à nouveau. |

Exemples :

```text
obit init -g                 # Claude Code, for every project (~/.claude)
obit init                    # Claude Code, this project only (.claude/)
obit init -g --agent codex   # Codex (~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall     # remove only what Obituary added
obit init -g --show          # what is installed
obit init --show --format markdown > docs/agents.md
```

### `obit list`
Alias: `list`, `ls`
Usage: `obit list [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

### `obit log`
Usage: `obit log [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |
| `--lines` | `-n` | `20` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit mcp`
Usage: `obit mcp [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit mktemp`
Usage: `obit mktemp <lifespan> [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |
| `--name` | aucun | `"tmp"` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--session` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

Exemples :

```text
obit mktemp 2h --name screenshots
```

### `obit proposals`
Usage: `obit proposals [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

### `obit purge`
Alias: `purge`, `cremate`
Usage: `obit purge [id\|original path]... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--all` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--dry-run` | aucun | `false` | Affiche seulement les actions prévues, sans modifier les fichiers ni les services. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |
| `--yes` | aucun | `false` | Confirme l’opération sans demander à nouveau. |

### `obit reject`
Usage: `obit reject <proposal id>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

### `obit remove-item`
Usage: `obit remove-item [-Recurse] [-Force] [-Path\|-LiteralPath] <path> [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit resurrect`
Alias: `resurrect`, `raise`
Usage: `obit resurrect <id\|original path>... \| --last [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |
| `--last` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit rm`
Usage: `obit rm [options] <path>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit rmdir`
Usage: `obit rmdir [options] <path>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit rules`
Usage: `obit rules [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

### `obit rules apply`
Usage: `obit rules apply <path>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |
| `--session` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit rules check`
Usage: `obit rules check <path>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |
| `--session` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit stats`
Alias: `stats`, `eulogy`
Usage: `obit stats [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

### `obit sweep`
Alias: `sweep`, `reap`
Usage: `obit sweep [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--dry-run` | aucun | `false` | Affiche seulement les actions prévues, sans modifier les fichiers ni les services. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |

### `obit trust`
Usage: `obit trust [rules file] [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--yes` | aucun | `false` | Confirme l’opération sans demander à nouveau. |

### `obit uninstall`
Usage: `obit uninstall [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--dry-run` | aucun | `false` | Affiche seulement les actions prévues, sans modifier les fichiers ni les services. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--purge-data` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--yes` | aucun | `false` | Confirme l’opération sans demander à nouveau. |

### `obit untrust`
Usage: `obit untrust [rules file] [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Affiche l’aide. |

### `obit update`
Usage: `obit update [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--apply` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--check` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--relaunch-gui` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--wait-pid` | aucun | `0` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

### `obit watch`
Usage: `obit watch <directory> <lifespan> \| watch --list \| watch --remove <directory> [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--age` | aucun | `"first-seen"` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--clear-exclusion` | aucun | `0` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--help` | `-h` | `false` | Affiche l’aide. |
| `--json` | aucun | `false` | Affiche la sortie en JSON. |
| `--list` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--no-daemon` | aucun | `false` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--policy` | aucun | `0` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--remove` | aucun | `""` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |
| `--remove-id` | aucun | `0` | Le comportement actuel de ce paramètre est généré depuis le code ; consultez aussi l’aide. |

<!-- generated-command-reference:end -->
