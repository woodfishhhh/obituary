# Référence de configuration

Obituary range ses réglages dans `config.toml`, dans le répertoire de configuration. Le fichier est facultatif : sans lui, ou avec seulement quelques lignes, tout le reste prend sa valeur par défaut. `obit config` vous indique où se trouve le fichier, s’il existe et la valeur que chaque réglage a réellement en ce moment. `obit config --create` écrit un fichier commenté contenant toutes les valeurs par défaut, ce qui est le plus simple pour commencer à modifier.

Le répertoire de configuration est `%APPDATA%\obituary` sous Windows, `~/Library/Application Support/obituary` sous macOS et `~/.config/obituary` sous Linux (ou `obituary` dans `XDG_CONFIG_HOME` si cette variable est définie). Pour le placer ailleurs, définissez la variable d’environnement `OBIT_CONFIG_DIR`.

| Clé | Valeur par défaut | Valeurs acceptées |
| --- | --- | --- |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` |
| `graveyard.retention_days` | `7` | un entier de 1 à 3650 |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` |
| `hooks.session_cleanup` | `true` | `true`, `false` |
| `hooks.session_summary` | `false` | `true`, `false` |
| `watch.stable_window` | `10m` | une durée positive, comme `30s`, `10m`, `1h30m` |
| `watch.file_expiry` (macOS/Linux) | `quiet` | `quiet`, `propose` |
| `explorer.enabled` | `true` | `true`, `false` |
| `explorer.everywhere` | `false` | `true`, `false` |
| `explorer.in_git` | `false` | `true`, `false` |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` |
| `ui.tray` | `true` | `true`, `false` |
| `ui.taskbar` | `true` | `true`, `false` |
| `update.check` | `true` | `true`, `false` |
| `telemetry.enabled` | `true` | `true`, `false` |
| `agent_shells` | non définie | une table : nom de l’agent = `bash` ou `powershell` |
| `codex_windows_shell` | non définie | `bash`, `powershell` |

## Où va ce qui expire

Par défaut, un élément dont la durée de vie est écoulée entre dans le cimetière d’Obituary. Pendant `graveyard.retention_days` jours (7 si vous ne changez rien), vous pouvez le remettre à sa place avec `obit resurrect` ; c’est seulement après ce délai qu’il est supprimé définitivement. Ce nombre peut aller de 1 à 3650.

Sous Windows et Linux, `graveyard.mode = "recycle"` utilise la Corbeille du système. Restaurez les fichiers depuis celle-ci, et non avec `obit resurrect`. Sous macOS, seul le cimetière est disponible.

## Enregistrement automatique et sessions d’agent

`proposals.mode` décide si Obituary donne de lui-même une durée de vie aux fichiers. La valeur par défaut, `off`, ne fait rien automatiquement : seul ce qu’un agent ou vous enregistrez par des commandes, le menu contextuel ou des règles de dossier entre. Avec `tiered`, les fichiers que l’agent a créés pendant cette session et les conteneurs que la session a démarrés entrent aussitôt et vous en êtes informé ensuite ; `obit spare` permet d’en retirer un. Tout le reste attend votre approbation dans `obit proposals`. `auto` enregistre tout ce que l’enregistrement automatique trouve et vous prévient ensuite. `ask` vous interroge d’abord sur chaque élément. Dans tous les modes, les fichiers que Git suit déjà, les dépendances, les sorties de compilation et les caches (`node_modules`, `dist`, `__pycache__` et autres) ne sont jamais enregistrés automatiquement. Les règles sont décrites dans [Quels fichiers enregistrer](./auto-register).

`hooks.session_cleanup` est activé par défaut. Vers la fin de chaque session, l’agent est informé des fichiers qu’il a créés qui n’ont pas de durée de vie et que Git ne suit pas, et il lui est demandé de donner une durée de vie à ses fichiers temporaires (`obit mark`) ou de les enterrer ; les livrables restent. `hooks.session_summary` est désactivé par défaut ; une fois activé, l’agent apprend aussi, au début de la session, quels éléments du projet expirent dans les 24 heures et ce qui a été enterré récemment. Il est désactivé pour économiser des jetons, car l’agent est de toute façon prévenu quand il touche un chemin enterré.

## Règles de dossier et dates dans l’Explorateur

`watch.stable_window` est la patience d’une règle de dossier : un nouveau fichier doit rester inchangé pendant cette durée avant d’être déplacé, afin qu’un fichier encore en cours de téléchargement ne soit pas emporté. Elle utilise le format de durée de Go, un nombre suivi de `s`, `m` ou `h`, que l’on peut enchaîner comme dans `1h30m`, et elle doit être supérieure à zéro.

Les réglages `explorer.*` ne comptent que sous Windows. Obituary fait afficher à l’Explorateur la date d’enterrement à côté du nom du fichier, sans changer le nom réel. Par défaut, cela se produit dans Téléchargements, sur le Bureau (sous-dossiers compris) et dans les emplacements gérés par une règle de dossier. Avec `explorer.everywhere` activé, les autres emplacements suivis l’affichent aussi ; les espaces de travail Git sont exclus, sauf si vous mettez `explorer.in_git` à `true` ; et `explorer.enabled = false` le désactive complètement. Voir [Règles de dossier](./folder-policies).

## Interface

`ui.language` définit la langue de la ligne de commande, de la fenêtre, de l’icône de notification et du menu contextuel. Seules `en` et `zh-CN` existent pour l’instant. `auto` consulte la variable d’environnement `OBIT_LANG`, puis `LC_ALL`, `LC_MESSAGES` et `LANG`, et enfin la langue d’interface du système. `ui.tray` affiche ou masque l’icône de notification (l’icône de la barre des menus sous macOS) ; masquée, elle laisse le nettoyage en arrière-plan et les vérifications de mise à jour se poursuivre. `ui.taskbar` ne s’applique qu’à Windows et décide si la fenêtre occupe un bouton de la barre des tâches tant qu’elle est ouverte.

## Vérification des mises à jour et statistiques d’utilisation

Lorsque `update.check` est activé, Obituary lit les informations publiques sur les versions de GitHub au plus une fois par jour et n’envoie aucune donnée d’utilisation. Une fois désactivé, vous pouvez toujours lancer `obit update --check` quand vous le souhaitez. `telemetry.enabled` commande les statistiques d’utilisation anonymes. C’est un interrupteur distinct de celui des mises à jour : `obit config telemetry off` désactive seulement les statistiques et efface de cet ordinateur l’identifiant d’installation et tout rapport en attente. Les détails figurent dans la [politique de confidentialité](./privacy).

## Quel shell un agent utilise sous Windows

La garde de suppression doit lire la commande de suppression qu’un agent s’apprête à exécuter pour proposer la commande `obit` équivalente. Sous Windows, un agent peut utiliser bash ou PowerShell, et Obituary ne peut pas le deviner : c’est donc vous qui le déclarez. Cet agent s’exécute sur cette machine Windows, avec ce shell. Sans déclaration, les commandes dont le sens est ambigu passent sans être bloquées.

```toml
[agent_shells]
codex = "powershell"
```

Les clés de `agent_shells` sont des noms d’agents. Pour le moment, `codex`, `codebuddy`, `codebuddy-ide` et `minimax-code` la lisent. `codex_windows_shell` est l’ancienne forme, réservée à Codex ; si elle et `agent_shells.codex` sont définies avec des valeurs différentes, la configuration est rejetée.

## À quoi ressemble une erreur

Obituary n’ignore pas en silence une clé qu’il ne connaît pas. S’il le faisait, `retention = 30` laisserait en vigueur la rétention de 7 jours alors que vous croiriez l’avoir portée à 30. Aussi `obit config` signale-t-il le chemin du fichier, le numéro de ligne et le nom complet de la clé non reconnue, et propose la bonne clé quand l’orthographe est proche :

```text
obit: config file has errors: …\config.toml:2: unknown key "graveyard.retention" (did you mean "graveyard.retention_days"?)
```

Une valeur hors limites est signalée de la même façon, avec le réglage et ce qu’il accepte, par exemple `graveyard.retention_days must be between 1 and 3650`. Une erreur de syntaxe TOML fait aussi échouer `obit config`. Corrigez le fichier et relancez la commande ; quand elle ne se plaint plus, c’est réparé.

## Commandes courantes

```powershell
obit config                  # emplacement du fichier, répertoire de données et valeurs en vigueur
obit config --create         # écrit le fichier par défaut commenté
obit config --language en    # enregistre la langue de l’interface : auto, en ou zh-CN
obit config telemetry off    # désactive les statistiques d’utilisation (on les réactive)
```

`--create` n’écrase jamais un fichier existant ; si le fichier est déjà là, la commande signale une erreur et s’arrête. `--language` crée le fichier avec les valeurs par défaut quand il n’existe pas encore.
