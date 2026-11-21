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

### `obit`

Obituary (讣告), la corbeille des agents IA : donne une durée de vie aux fichiers ; une fois celle-ci écoulée, ils sont enterrés dans un cimetière d’où l’on peut les ressusciter

Usage: `obit [command]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour obit |
| `--version` | `-v` | `false` | Version d’obit |

### `obit add`

Donner une durée de vie à des chemins (réenregistrer un chemin suivi réinitialise son échéance)

Alias: `add`, `mark`
Usage: `obit add <path>... <duration> | add <path>... --at <time> [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--at` | aucun | `""` | Expirer à une heure locale donnée : "2006-01-02 15:04", "2006-01-02" ou "15:04" |
| `--force` | aucun | `false` | Autoriser les dossiers qui contiennent un dépôt .git |
| `--help` | `-h` | `false` | Aide pour add |
| `--json` | aucun | `false` | Afficher le JSON |
| `--no-daemon` | aucun | `false` | Ne pas démarrer le démon en arrière-plan |
| `--note` | aucun | `""` | Note libre affichée dans la liste |

Exemples :

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`

Approuver des propositions : enregistrer ce qu’elles listent

Usage: `obit approve <proposal id>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour approve |
| `--json` | aucun | `false` | Afficher le JSON |

### `obit bury`

Déplacer des chemins vers le cimetière maintenant (restauration avec obit resurrect)

Usage: `obit bury <path>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--force` | aucun | `false` | Autoriser les dossiers qui contiennent un dépôt .git |
| `--help` | `-h` | `false` | Aide pour bury |
| `--json` | aucun | `false` | Afficher le JSON |
| `--note` | aucun | `""` | Note libre conservée avec l’enterrement |

### `obit cancel`

Épargner des chemins ou des ID de conteneur : arrêter leur suivi et conserver les objets

Alias: `cancel`, `spare`
Usage: `obit cancel <id|path>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour cancel |
| `--json` | aucun | `false` | Afficher le JSON |

### `obit completion`

Générer le script d’autocomplétion d’obit pour le shell indiqué.

Usage: `obit completion [command]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour completion |

### `obit completion bash`

Générer le script d’autocomplétion pour le shell bash.

Usage: `obit completion bash`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour bash |
| `--no-descriptions` | aucun | `false` | Désactiver les descriptions de l’autocomplétion |

### `obit completion fish`

Générer le script d’autocomplétion pour le shell fish.

Usage: `obit completion fish [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour fish |
| `--no-descriptions` | aucun | `false` | Désactiver les descriptions de l’autocomplétion |

### `obit completion powershell`

Générer le script d’autocomplétion pour powershell.

Usage: `obit completion powershell [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour powershell |
| `--no-descriptions` | aucun | `false` | Désactiver les descriptions de l’autocomplétion |

### `obit completion zsh`

Générer le script d’autocomplétion pour le shell zsh.

Usage: `obit completion zsh [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour zsh |
| `--no-descriptions` | aucun | `false` | Désactiver les descriptions de l’autocomplétion |

### `obit config`

Afficher le fichier de configuration et les paramètres effectifs (--create écrit les valeurs par défaut)

Usage: `obit config [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--create` | aucun | `false` | Écrire un fichier de configuration avec les valeurs par défaut |
| `--help` | `-h` | `false` | Aide pour config |
| `--language` | aucun | `""` | Définir la langue : auto, en ou zh-CN |

### `obit config telemetry`

Statistiques d’utilisation : un ID d’installation aléatoire, la version, le système d’exploitation, l’architecture, la langue de l’interface et le canal d’installation servent à mesurer le nombre d’installations et l’usage des versions. Aucun fichier, chemin ni commande n’est envoyé ; les adresses IP et les agents utilisateurs ne sont pas conservés. Désactivation avec obit config telemetry off ou OBIT_TELEMETRY=0.

Usage: `obit config telemetry on|off [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour telemetry |

### `obit daemon`

Contrôler le démon en arrière-plan (obitd)

Usage: `obit daemon [command]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour daemon |

### `obit daemon disable`

Supprimer la tâche planifiée et le démon résident

Usage: `obit daemon disable [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--dry-run` | aucun | `false` | Afficher seulement les changements |
| `--help` | `-h` | `false` | Aide pour disable |

### `obit daemon enable`

Activer le planificateur utilisateur de 5 minutes (et, sous Windows, l’icône de la zone de notification lancée à l’ouverture de session)

Usage: `obit daemon enable [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--dry-run` | aucun | `false` | Afficher seulement les changements |
| `--help` | `-h` | `false` | Aide pour enable |
| `--resident` | aucun | `false` | Activer l’icône de la zone de notification sans modifier la tâche planifiée de secours |

### `obit daemon start`

Démarrer le démon maintenant

Usage: `obit daemon start [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour start |

### `obit daemon status`

Afficher l’état du démon et la prochaine échéance

Usage: `obit daemon status [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour status |
| `--json` | aucun | `false` | Afficher le JSON |

### `obit daemon stop`

Arrêter le démon

Usage: `obit daemon stop [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour stop |

### `obit docker`

Suivre les conteneurs Docker par leur ID complet immuable et leur date de création. À l’expiration, le conteneur est arrêté et sa couche accessible en écriture est conservée. La résurrection le démarre. Une fois écoulés les jours de [graveyard] retention_days, la suppression définitive retire les conteneurs arrêtés sans toucher aux volumes ; cela vaut aussi en mode corbeille. Les images, les volumes et les réseaux sont conservés.

Usage: `obit docker [command]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour docker |

### `obit docker add`

Enregistrer un conteneur Docker ; l’expiration l’arrête

Usage: `obit docker add <container name|ID> <lifespan> [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour add |
| `--json` | aucun | `false` | Afficher le JSON |

### `obit docker list`

Lister les conteneurs Docker enregistrés

Usage: `obit docker list [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour list |
| `--json` | aucun | `false` | Afficher le JSON |

### `obit doctor`

Vérifier l’installation, les intégrations et les données sans rien modifier

Usage: `obit doctor [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour doctor |
| `--json` | aucun | `false` | Afficher le JSON |
| `--verbose` | aucun | `false` | Inclure les détails des capacités des agents |

### `obit extend`

Prolonger : repousser les échéances (comptées à partir de maintenant si elles sont déjà dépassées)

Usage: `obit extend <id|path>... <duration> [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour extend |
| `--json` | aucun | `false` | Afficher le JSON |

Exemples :

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`

Enterrer les chemins sélectionnés par la simulation de git clean

Usage: `obit git-clean [options] [--] [pathspec]... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour git-clean |

### `obit graveyard`

Lister ce que contient le cimetière et qui peut encore être ressuscité

Alias: `graveyard`, `graves`
Usage: `obit graveyard [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour graveyard |
| `--here` | aucun | `false` | Afficher seulement les chemins d’origine situés dans le répertoire courant et ses sous-répertoires |
| `--json` | aucun | `false` | Afficher le JSON |

### `obit help`

Afficher l’aide de n’importe quelle commande de l’application.

Usage: `obit help [command] [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour help |

### `obit hook`

Points d’entrée des hooks d’agent (appelés par l’agent, pas par des personnes)

Usage: `obit hook [command]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour hook |

### `obit hook antigravity`

Traiter un événement de hook antigravity depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook antigravity [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour antigravity |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook audit`

Afficher les pistes d’audit des hooks et de la garde de suppression

Usage: `obit hook audit [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour audit |
| `--lines` | `-n` | `40` | Nombre de lignes par source |

### `obit hook claude`

Traiter un événement de hook claude depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook claude [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour claude |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook cline`

Traiter un événement de hook cline depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook cline [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour cline |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook codebuddy`

Traiter un événement de hook codebuddy depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook codebuddy [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour codebuddy |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook codebuddy-ide`

Traiter un événement de hook codebuddy-ide depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook codebuddy-ide [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour codebuddy-ide |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook codex`

Traiter un événement de hook codex depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook codex [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour codex |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook copilot`

Traiter un événement de hook copilot depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook copilot [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour copilot |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook cursor`

Traiter un événement de hook cursor depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook cursor [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour cursor |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook deepseek-harness`

Traiter un événement de hook deepseek-harness depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook deepseek-harness [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour deepseek-harness |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook droid`

Traiter un événement de hook droid depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook droid [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour droid |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook gemini`

Traiter un événement de hook gemini depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook gemini [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour gemini |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook hermes`

Traiter un événement de hook hermes depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook hermes [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour hermes |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook kilocode`

Traiter un événement de hook kilocode depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook kilocode [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour kilocode |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook kimi`

Traiter un événement de hook kimi depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook kimi [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour kimi |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook lingma`

Traiter un événement de hook lingma depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook lingma [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour lingma |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook minimax-code`

Traiter un événement de hook minimax-code depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook minimax-code [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour minimax-code |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook omp`

Traiter un événement de hook omp depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook omp [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour omp |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook openclaw`

Traiter un événement de hook openclaw depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook openclaw [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour openclaw |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook opencode`

Traiter un événement de hook opencode depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook opencode [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour opencode |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook pi`

Traiter un événement de hook pi depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook pi [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour pi |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook qoder`

Traiter un événement de hook qoder depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook qoder [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour qoder |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook qoder-cn`

Traiter un événement de hook qoder-cn depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook qoder-cn [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour qoder-cn |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook trae`

Traiter un événement de hook trae depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook trae [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour trae |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook trae-cn`

Traiter un événement de hook trae-cn depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook trae-cn [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour trae-cn |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook vibe`

Traiter un événement de hook vibe depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook vibe [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour vibe |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook zcode`

Traiter un événement de hook zcode depuis stdin ; en cas d’erreur, l’action est autorisée

Usage: `obit hook zcode [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--event` | aucun | `""` | Nom de l’événement de l’hôte lorsqu’il est absent de la charge utile |
| `--help` | `-h` | `false` | Aide pour zcode |
| `--notice-only` | aucun | `false` | Fournir un contexte indicatif sans la garde de suppression |

### `obit hook finish`

Calculer les fichiers laissés par une session (lancé en arrière-plan par les hooks)

Usage: `obit hook finish [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour finish |
| `--project` | aucun | `""` | Racine du projet |
| `--session` | aucun | `""` | ID de la session de l’agent |

### `obit init`

Installer (ou désinstaller avec --uninstall) les hooks et les instructions d’Obituary pour un agent

Usage: `obit init [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--agent` | aucun | `[claude]` | Agents à intégrer (répéter l’option ou séparer par des virgules ; par défaut : claude) |
| `--context-menu` | aucun | `false` | Installer les menus contextuels classiques de l’Explorateur (activé par défaut) |
| `--dry-run` | aucun | `false` | Afficher seulement les changements |
| `--format` | aucun | `"text"` | Format de sortie de --show : text ou markdown (couverture des agents) |
| `--global` | `-g` | `false` | Pour tout l’utilisateur plutôt que pour ce seul projet |
| `--guard-rm` | aucun | `false` | Refuser les commandes de suppression prises en charge et proposer des équivalents obit restaurables |
| `--help` | `-h` | `false` | Aide pour init |
| `--mcp` | aucun | `false` | Enregistrer le serveur MCP stdio d’Obituary |
| `--no-context-menu` | aucun | `false` | Ne pas installer les menus contextuels de l’Explorateur |
| `--no-shortcuts` | aucun | `false` | Ne pas créer les raccourcis du menu Démarrer et du Bureau |
| `--no-skill` | aucun | `false` | Ne pas installer l’Agent Skill d’Obituary |
| `--show` | aucun | `false` | Afficher ce qui est installé |
| `--skill` | aucun | `false` | Installer l’Agent Skill d’Obituary (activé par défaut) |
| `--uninstall` | aucun | `false` | Retirer ce qu’Obituary a ajouté |
| `--yes` | aucun | `false` | Appliquer sans demander |

Exemples :

```text
obit init -g  # Claude Code, pour tous les projets (~/.claude)
obit init  # Claude Code, ce projet uniquement (.claude/)
obit init -g --agent codex  # Codex (~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall  # Retirer uniquement ce qu’Obituary a ajouté
obit init -g --show  # Ce qui est installé
obit init --show --format markdown > docs/agents.md
```

### `obit list`

Afficher les chemins suivis et les conteneurs Docker, du plus proche de l’échéance au plus lointain

Alias: `list`, `ls`
Usage: `obit list [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour list |
| `--json` | aucun | `false` | Afficher le JSON |

### `obit log`

Afficher le journal d’audit de tout ce qu’Obituary a fait

Usage: `obit log [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour log |
| `--json` | aucun | `false` | Afficher le JSON |
| `--lines` | `-n` | `20` | Nombre d’événements |

### `obit mcp`

Exposer les outils d’Obituary via MCP stdio

Usage: `obit mcp [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour mcp |

### `obit mktemp`

Créer un répertoire temporaire qui s’enterre tout seul et afficher son chemin

Usage: `obit mktemp <lifespan> [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour mktemp |
| `--json` | aucun | `false` | Afficher le JSON |
| `--name` | aucun | `"tmp"` | Préfixe du nom du répertoire |
| `--session` | aucun | `""` | Session d’agent à laquelle appartient le répertoire |

Exemples :

```text
dir=$(obit mktemp 1d)
obit mktemp 2h --name screenshots
```

### `obit proposals`

Enregistrements en attente de votre approbation, et ceux effectués sans demander

Usage: `obit proposals [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour proposals |
| `--json` | aucun | `false` | Afficher le JSON |

### `obit purge`

Supprimer les enterrements de fichiers sélectionnés ou dont la conservation est écoulée. --all et les références explicites ignorent le délai de conservation ; ils demandent donc confirmation d’abord, et hors d’un terminal ils exigent --yes. Les conteneurs Docker exigent que [graveyard] retention_days soit écoulé, y compris avec --all ou un ID explicite ; les conteneurs en cours d’exécution sont considérés comme ressuscités. La suppression Docker préserve les volumes et ne force jamais la suppression. Le traitement des conteneurs est identique en mode corbeille.

Alias: `purge`, `cremate`
Usage: `obit purge [id|original path]... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--all` | aucun | `false` | Supprimer définitivement tous les enterrements, pas seulement ceux dont la conservation est écoulée |
| `--dry-run` | aucun | `false` | Afficher seulement ce qui se passerait |
| `--help` | `-h` | `false` | Aide pour purge |
| `--json` | aucun | `false` | Afficher le JSON |
| `--yes` | aucun | `false` | Supprimer définitivement --all ou les enterrements sélectionnés sans demander (agents : demander d’abord à l’utilisateur) |

### `obit reject`

Rejeter des propositions

Usage: `obit reject <proposal id>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour reject |
| `--json` | aucun | `false` | Afficher le JSON |

### `obit remove-item`

Enterrer des éléments du système de fichiers avec un sous-ensemble limité de paramètres PowerShell.

Usage: `obit remove-item [-Recurse] [-Force] [-Path|-LiteralPath] <path> [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour remove-item |

### `obit resurrect`

Restaurer des fichiers sans écraser ; démarrer par ID les conteneurs Docker enterrés

Alias: `resurrect`, `raise`
Usage: `obit resurrect <id|original path>... | --last [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour resurrect |
| `--json` | aucun | `false` | Afficher le JSON |
| `--last` | aucun | `false` | Restaurer le dernier lot de suppressions de la session en cours ou du projet Git |

### `obit rm`

Déplacer des chemins vers le cimetière maintenant (restauration avec obit resurrect)

Usage: `obit rm [options] <path>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour rm |

### `obit rmdir`

Déplacer des chemins vers le cimetière maintenant (restauration avec obit resurrect)

Usage: `obit rmdir [options] <path>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour rmdir |

### `obit rules`

Afficher les règles en vigueur ici : celles qui sont intégrées, plus le .obitrules du projet dès qu’on lui fait confiance

Usage: `obit rules [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour rules |
| `--json` | aucun | `false` | Afficher le JSON |

### `obit rules apply`

Enregistrer des chemins selon les règles (les fichiers suivis par Git ne sont jamais enregistrés)

Usage: `obit rules apply <path>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour apply |
| `--json` | aucun | `false` | Afficher le JSON |
| `--session` | aucun | `""` | Session d’agent d’où viennent les chemins |

### `obit rules check`

Afficher ce que les règles enregistreraient pour des chemins (ne modifie rien)

Usage: `obit rules check <path>... [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour check |
| `--json` | aucun | `false` | Afficher le JSON |
| `--session` | aucun | `""` | Session d’agent d’où viennent les chemins |

### `obit stats`

Ce qu’Obituary a fait : éléments suivis, enterrés, espace récupéré, résurrections

Alias: `stats`, `eulogy`
Usage: `obit stats [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour stats |
| `--json` | aucun | `false` | Afficher le JSON |

### `obit sweep`

Récolter maintenant : enterrer tout ce qui est arrivé à échéance sans attendre le processus en arrière-plan

Alias: `sweep`, `reap`
Usage: `obit sweep [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--dry-run` | aucun | `false` | Afficher seulement ce qui se passerait |
| `--help` | `-h` | `false` | Aide pour sweep |
| `--json` | aucun | `false` | Afficher le JSON |

### `obit trust`

Examiner le .obitrules du projet et lui faire confiance (toute modification impose un nouvel examen)

Usage: `obit trust [rules file] [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour trust |
| `--yes` | aucun | `false` | Faire confiance sans demander (seulement après avoir examiné les règles) |

### `obit uninstall`

Retirer les intégrations et les services en arrière-plan d’Obituary ; les données sont conservées par défaut

Usage: `obit uninstall [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--dry-run` | aucun | `false` | Afficher seulement le plan de désinstallation ; ne modifier ni fichiers ni services |
| `--help` | `-h` | `false` | Aide pour uninstall |
| `--purge-data` | aucun | `false` | Supprimer définitivement les tombes listées et retirer les données appartenant à Obituary ; conserver les fichiers inconnus |
| `--yes` | aucun | `false` | Confirmer le plan (et la suppression définitive des données seulement si --purge-data est aussi indiqué) |

### `obit untrust`

Cesser d’appliquer le .obitrules du projet

Usage: `obit untrust [rules file] [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | Aide pour untrust |

### `obit update`

Vérifier ou installer une mise à jour stable

Usage: `obit update [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--apply` | aucun | `false` | Télécharger, vérifier et installer la version stable plus récente mise en cache (une installation manuelle ne remplace que les programmes d’Obituary) |
| `--check` | aucun | `false` | Vérifier GitHub maintenant, même si les vérifications automatiques sont désactivées |
| `--help` | `-h` | `false` | Aide pour update |
| `--relaunch-gui` | aucun | `false` | Rouvrir la petite fenêtre après la mise à jour (Windows) |
| `--wait-pid` | aucun | `0` | Attendre la fin de ce processus avant la mise à jour (Windows) |

### `obit watch`

Donner à chaque élément de premier niveau d’un dossier sa propre durée de vie

Usage: `obit watch <directory> <lifespan> | watch --list | watch --remove <directory> [flags]`

| Paramètre | Court | Valeur par défaut | Description |
| --- | --- | --- | --- |
| `--age` | aucun | `"first-seen"` | Base de l’âge : first-seen ou modified (nécessite une approbation) |
| `--clear-exclusion` | aucun | `0` | Effacer explicitement cet ID d’exclusion (avec --policy) |
| `--help` | `-h` | `false` | Aide pour watch |
| `--json` | aucun | `false` | Afficher le JSON |
| `--list` | aucun | `false` | Lister les règles de dossier |
| `--no-daemon` | aucun | `false` | Ne pas démarrer le processus en arrière-plan |
| `--policy` | aucun | `0` | ID de la règle dont l’exclusion doit être effacée |
| `--remove` | aucun | `""` | Retirer une règle et annuler les durées de vie qu’elle a enregistrées |
| `--remove-id` | aucun | `0` | Retirer une règle de dossier par ID, y compris pour les chemins manquants |

<!-- generated-command-reference:end -->
