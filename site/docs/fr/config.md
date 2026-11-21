# Référence de configuration

Obituary lit `config.toml` dans son répertoire de configuration. Toutes les clés sont facultatives et les clés absentes utilisent les valeurs par défaut ci-dessous. `obit config` affiche le chemin et les valeurs effectives.

## Clés

| Clé | Valeur par défaut | Valeurs ou format | Effet |
| --- | --- | --- | --- |
| `codex_windows_shell` | non défini | `bash`, `powershell` | Déclare le shell de Codex sous Windows ; une suppression ambiguë passe telle quelle si la clé manque. |
| `agent_shells.<agent>` | non défini | `bash`, `powershell` | Déclare le shell Windows d’un hook agent. |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` | Déplace les éléments échus au cimetière ou dans la corbeille système. |
| `graveyard.retention_days` | `7` | Entier de `1` à `3650` | Nombre de jours pendant lesquels un élément reste restaurable. |
| `watch.stable_window` | `10m` | Durée Go, par exemple `30s`, `10m`, `1h` | Attend la stabilisation d’un élément avant le déplacement par une règle de dossier. |
| `explorer.enabled` | `true` | `true`, `false` | Écrit les étiquettes de date de l’Explorateur. |
| `explorer.everywhere` | `false` | `true`, `false` | Affiche les dates dans tous les dossiers compatibles. |
| `explorer.in_git` | `false` | `true`, `false` | Affiche les dates dans les worktrees Git. |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` | Langue du CLI, de la fenêtre et de la barre de menus. |
| `ui.tray` | `true` | `true`, `false` | Active l’entrée de la barre système ou de la barre de menus. |
| `ui.taskbar` | `true` | `true`, `false` | Affiche un bouton séparé pour la fenêtre Windows. |
| `hooks.session_summary` | `false` | `true`, `false` | Affiche un résumé au début d’une session. |
| `hooks.session_cleanup` | `true` | `true`, `false` | Range les fichiers créés par cette session qui restent non classés. |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` | Définit les enregistrements automatiques qui attendent une approbation. |
| `update.check` | `true` | `true`, `false` | Vérifie automatiquement les versions stables. |
| `telemetry.enabled` | `true` | `true`, `false` | Autorise les statistiques d’utilisation ; ce réglage est indépendant des mises à jour. |

`agent_shells` est une table indexée par nom d’agent, par exemple `[agent_shells] codex = "powershell"`. `codex_windows_shell` est l’ancienne forme pour Codex ; des valeurs contradictoires provoquent une erreur.

## Formats et erreurs

Les durées suivent la syntaxe Go et peuvent combiner `ns`, `us`, `ms`, `s`, `m` et `h`, par exemple `1h30m`. La rétention doit être comprise entre `1` et `3650`. `graveyard.mode`, `ui.language` et `proposals.mode` n’acceptent que les valeurs indiquées.

Les clés inconnues ne sont pas ignorées. L’erreur indique le chemin, la ligne et la clé complète, avec une suggestion si l’orthographe est proche. Une erreur TOML fait également échouer `obit config`.

## Commandes courantes

```powershell
obit config
obit config --create
obit config --language en
obit config telemetry off
obit config telemetry on
```

`--create` écrit le modèle commenté uniquement si le fichier n’existe pas. `--language` accepte `auto`, `en` et `zh-CN`. `telemetry on|off` ne change que `telemetry.enabled` et ne modifie pas `[update] check`.
