# Désinstallation

Désinstaller se fait en deux temps : Obituary retire d’abord ce qu’il a mis en place, puis vous supprimez les programmes eux-mêmes. Le cimetière et le registre sont conservés par défaut, de sorte que les fichiers que vous avez enterrés restent récupérables ensuite.

```powershell
obit uninstall --dry-run      # liste ce qui serait retiré, sans rien modifier
obit uninstall                # affiche le plan, puis l’applique après votre confirmation
```

Si vous avez installé avec le script sous Windows, une seule commande suffit :

```powershell
.\install.ps1 -Uninstall      # lance obit uninstall --yes à votre place, puis supprime les fichiers du programme, l’entrée PATH qu’il a ajoutée et son raccourci du menu Démarrer
```

## Ce qui est retiré

`obit uninstall` agit au niveau de l’utilisateur et sur le projet Git courant : les hooks, MCP et Skill de chaque agent, la tâche planifiée qui s’exécute toutes les 5 minutes (le Planificateur de tâches sous Windows, un agent utilisateur launchd sous macOS, un minuteur utilisateur systemd sous Linux), le processus résident et l’icône de notification, ainsi que les règles de dossier (les fichiers qui en dépendent restent où ils sont). Sous Windows, il retire aussi l’entrée de démarrage à l’ouverture de session, le menu contextuel de l’Explorateur et les étiquettes de date de l’Explorateur.

Il ne touche qu’à ce qu’Obituary a lui-même écrit. Une entrée que vous avez modifiée, ou laissée par une autre installation, est conservée telle quelle, et le plan indique laquelle et pourquoi. Là où il n’y a pas de terminal (un script, la ligne de commande d’un agent), ajoutez `--yes` ; sans cela, la commande s’arrête et vous dit qu’il lui faut un terminal.

Trois choses ne sont pas faites pour vous. Les fichiers du programme se suppriment selon la manière dont vous avez installé (voir plus bas). Si vous avez installé des hooks au niveau d’un projet dans d’autres projets, lancez `obit uninstall` dans chacun d’eux avant de supprimer les programmes, sinon les hooks de ces projets échoueront parce que `obit` aura disparu. Enfin, sous macOS et Linux, l’entrée de démarrage de l’icône de notification ne figure pas dans la liste : avant de désinstaller, désactivez « Start the tray at sign-in » dans les Settings de la fenêtre, ce qui supprime l’entrée qu’Obituary a écrite. Sous Windows, `obit uninstall` s’occupe du démarrage de l’icône, mais les raccourcis du menu Démarrer et du Bureau créés par `obit init` ne sont pas dans sa liste ; `obit init --uninstall` les supprime, sauf si vous ajoutez `--no-shortcuts`.

## Le cimetière, le registre et votre configuration

Trois choses sont conservées par défaut : les fichiers du cimetière, le registre et le journal d’audit, et `config.toml`. `obit uninstall` vous dit ce qui reste dans le cimetière et où il se trouve, et une réinstallation ultérieure retrouve le tout. Si vous utilisez le mode Corbeille, ce qui a expiré est déjà dans la Corbeille du système ; la désinstallation n’y touche pas, et on le restaure depuis la Corbeille, car `obit resurrect` ne peut pas l’atteindre.

Quand vous voulez vraiment tout effacer, ajoutez `--purge-data` :

```powershell
obit uninstall --purge-data
```

La commande liste d’abord chaque élément du cimetière et vous redemande une confirmation après celle du plan de désinstallation. Une fois cela confirmé, et avant de retirer quoi que ce soit, Obituary vérifie tout ce qu’il s’apprête à toucher : que le répertoire de données lui appartient, qu’il reconnaît chaque fichier qui s’y trouve, et qu’il peut vérifier le dossier du cimetière et chacune des tombes à purger. Si une vérification échoue, il s’arrête là et indique laquelle ; le cimetière, le registre et les services restent tels qu’ils étaient, de sorte qu’une purge n’est jamais faite à moitié. C’est seulement quand tout passe qu’il supprime ces fichiers définitivement, puis le registre, le journal d’audit et les autres fichiers qu’Obituary conserve. Dans un script, `--yes --purge-data` ensemble sautent les deux questions.

Elle ne supprime que les fichiers qu’elle reconnaît. D’autres fichiers, des liens ou un sous-dossier qu’Obituary ne connaît pas arrêtent la vérification, et la commande nomme celui qu’elle a trouvé. `config.toml` ne fait jamais partie de la purge et il n’est pas nécessaire de le déplacer, même lorsqu’il se trouve dans le même dossier que le registre (le cas par défaut sous macOS) : après la purge, ce dossier ne contient plus que `config.toml`. Quand la configuration est ailleurs, tout le répertoire de données est supprimé.

## Supprimer les programmes

La façon de supprimer les programmes dépend de la façon dont vous les avez installés.

| Installé avec | Se supprime avec |
| --- | --- |
| Le script d’installation Windows | `.\install.ps1 -Uninstall` |
| Scoop | `obit uninstall`, puis `scoop uninstall obituary` |
| winget | `obit uninstall`, puis `winget uninstall woodfishhhh.Obituary` |
| La fenêtre macOS (DMG) | `obit uninstall` et désactiver « Start the tray at sign-in », puis glisser Obituary.app d’Applications vers la Corbeille |
| Homebrew | `obit uninstall`, puis `brew uninstall --cask obituary` |
| `install.sh` sous macOS ou Linux | `obit uninstall`, puis supprimer `obit` et `obitd` dans `~/.local/bin` |
| Le paquet de bureau Ubuntu (`.deb`) | désactiver « Start the tray at sign-in » et quitter l’icône de notification, puis `sudo apt remove obituary-desktop` |

L’app macOS et le `.deb` Ubuntu apportent chacun leur propre `obit`, mais ni l’un ni l’autre ne le place dans votre PATH. Si vous n’avez pas installé la ligne de commande séparément, lancez-le avec son chemin complet avant de désinstaller : `/Applications/Obituary.app/Contents/MacOS/obit` pour l’app, `/usr/libexec/obituary/obit` pour le `.deb`. `apt remove` supprime les programmes de ce dossier et laisse votre cimetière, vos réglages et toute ligne de commande installée autrement.

## Où se trouvent les données

| | Données (registre, cimetière, journaux) | Configuration (`config.toml`) |
| --- | --- | --- |
| Windows | `%LOCALAPPDATA%\obituary` | `%APPDATA%\obituary` |
| macOS | `~/Library/Application Support/obituary` | le même dossier |
| Linux | `~/.local/share/obituary` | `~/.config/obituary` |

Sous Linux, si `XDG_DATA_HOME` ou `XDG_CONFIG_HOME` est définie, chacune désigne le dossier `obituary` qu’elle contient. Les variables d’environnement `OBIT_HOME` et `OBIT_CONFIG_DIR` déplacent les données et la configuration ailleurs, et `obit config` affiche celles qui sont utilisées. Sur le lecteur qui contient le répertoire de données, le cimetière est son dossier `graveyard` ; sur les autres lecteurs, c’est un dossier caché `.obituary` à la racine du lecteur, si bien qu’enterrer est toujours un simple renommage sur un même lecteur, jamais une copie.
