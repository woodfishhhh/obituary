# Désinstallation

```powershell
obit uninstall --dry-run      # see what would be removed
obit uninstall                # remove everything Obituary set up, after you confirm
.\install.ps1 -Uninstall      # if you used the install script: also remove the program files, PATH entry and Start menu shortcut
```

`obit uninstall` retire uniquement ce qu’Obituary a ajouté : les hooks des agents, MCP et la Skill installés pour l’utilisateur et pour le projet Git courant, le démarrage à l’ouverture de session, la tâche planifiée, le démon en cours d’exécution, le menu contextuel, les dates affichées dans l’Explorateur et les règles de dossier.

## Le cimetière et le registre sont conservés

Par défaut, le cimetière et le registre sont conservés. La commande indique où se trouve le cimetière et combien il contient. Pour les supprimer tous les deux, ajoutez `--purge-data` : la commande affiche d’abord le contenu du cimetière, puis demande une nouvelle confirmation.

## Hooks installés dans d’autres projets

Si vous avez installé des hooks au niveau du projet dans d’autres dépôts, exécutez `obit uninstall` dans chacun avant de désinstaller le programme. Sinon, ces hooks ne fonctionneront plus une fois `obit` supprimé.

## Scoop et winget

Exécutez d’abord `obit uninstall`, puis la commande de désinstallation du gestionnaire de paquets utilisé.
## Portée de la désinstallation et données

`obit uninstall --dry-run` liste les éléments supprimables pour l’utilisateur et le projet Git actuels. La commande retire hooks, MCP, Skills, barre système, démarrage et planificateurs, mais ne modifie pas les autres projets ; exécutez-la dans chacun.

| Élément | Windows | macOS | Linux |
| --- | --- | --- | --- |
| Programme | `obit`, `obitd`, `obit-gui` facultatif | `/Applications/Obituary.app` ou répertoire CLI | Répertoire CLI ; bureau en `.deb` |
| Hook, MCP, Skill | Configuration de l’agent | Configuration de l’agent | Configuration de l’agent |
| Planification | Tâche, processus résident et barre système | LaunchAgent et barre des menus | Timer systemd et autostart XDG |
| Projet | Parties ajoutées à `.claude/` et `.codex/` | Configuration du projet | Configuration du projet |

Les données par défaut sont `%LOCALAPPDATA%\obituary` et `%APPDATA%\obituary` sous Windows, `~/Library/Application Support/obituary` sous macOS et `$XDG_DATA_HOME/obituary` sous Linux (`~/.local/share/obituary` sinon). `OBIT_HOME`, `OBIT_CONFIG_DIR` et `OBIT_USER_HOME` redirigent ces emplacements ; un `.obituary` peut aussi se trouver sur le même volume.

Le cimetière, le registre, les journaux et la configuration sont conservés. `--purge-data` ne supprime que les données de cimetière dont la propriété est vérifiée et garde les fichiers inconnus. En mode recycle, restaurez depuis la corbeille, pas avec `resurrect`.
