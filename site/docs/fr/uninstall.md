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
