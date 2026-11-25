# Dépannage

Quand quelque chose ne va pas, commencez par `obit doctor`. La commande donne d’abord un résumé court : la première ligne dit si tout va bien ou combien de problèmes ont été trouvés, puis chaque domaine (installation, exécution en arrière-plan, intégrations d’agents, menu contextuel, données, hooks, etc.) occupe une ligne marquée `PASS`, `WARN` ou `FAIL`. Sous un domaine qui demande de l’attention figurent la raison et une commande de correction à copier telle quelle. À chaque exécution, le rapport complet est en outre enregistré sous la forme d’un fichier HTML autonome dans le dossier `reports` du répertoire de données (il ne quitte pas votre ordinateur et est enterré au bout de 1 jour) ; les dernières lignes indiquent où il se trouve. `obit doctor --open` l’ouvre dans le navigateur, et `obit doctor --verbose` affiche dans le terminal la version texte complète : une ligne par contrôle, qui commence par `PASS`, `INFO`, `WARN` ou `FAIL`, suivie de la raison et de ce qu’il faut faire, ainsi que des capacités de chaque agent. Dès qu’il y a un `FAIL`, la commande se termine avec un code de sortie non nul. Ajoutez `--json` pour transmettre le résultat à quelqu’un (son format n’a pas changé). Les lignes citées plus bas dans cette page, comme `path` ou `obitd`, sont les contrôles individuels que montrent `--verbose` et le rapport.

Chaque rubrique ci-dessous suit le même ordre : ce que vous voyez, pourquoi, ce qu’il faut faire et comment savoir que c’est réglé.

Après une mise à niveau, si tous les libellés du menu de l’Explorateur sont dans une autre langue prise en charge, le menu reste utilisable. `obit doctor` le signale à titre informatif (JSON : `installation: label-language-mismatch`). `obit init --context-menu --yes` applique la langue actuelle de l’interface.

## Le shell ne trouve pas `obit`

Le terminal affiche `command not found`, ou PowerShell ne reconnaît pas `obit`. Le programme est bien sur le disque, mais son dossier n’est pas dans votre PATH. Le script d’installation Windows ajoute `%LOCALAPPDATA%\Programs\obituary` au PATH de votre utilisateur, mais un terminal déjà ouvert ne voit pas le nouveau PATH. `install.sh` place les programmes dans `~/.local/bin` et ne modifie jamais le PATH, et sur certains systèmes ce dossier n’est de toute façon pas dans le PATH. Le `.deb` d’Ubuntu les place dans `/usr/libexec/obituary/`, qui n’est pas non plus dans le PATH.

Sous Windows, fermez le terminal et ouvrez-en un nouveau. Sous macOS et Linux, ajoutez `~/.local/bin` à la configuration de votre shell : avec zsh, mettez `export PATH="$HOME/.local/bin:$PATH"` dans `~/.zshrc`, puis ouvrez un nouveau terminal. Si vous n’avez installé que le `.deb`, utilisez le chemin complet `/usr/libexec/obituary/obit`, ou installez la ligne de commande séparément avec le script d’installation.

C’est réglé quand `obit --version` affiche une version et que la ligne `path` de `obit doctor` n’est plus en `WARN`.

## `obit doctor` ne trouve pas `obitd`

Une ligne `FAIL obitd` dans `obit doctor` signifie que le programme d’arrière-plan `obitd` n’est pas à côté de `obit`. Obituary le cherche dans le même dossier, donc copier `obit` seul ne suffit pas. Placez `obit` et `obitd` ensemble, ou relancez l’installation selon la méthode que vous avez employée. Quand `obit doctor` affiche `PASS` sur cette ligne, c’est réglé.

## Un fichier téléchargé échoue à la vérification de somme de contrôle

Avec le script d’installation, cette vérification est faite par le script lui-même, qui s’arrête sans rien installer si le hachage ne correspond pas. Vous n’avez à vérifier à la main que si vous avez téléchargé vous-même une archive, un DMG ou un `.deb`. Une différence signifie en général que le téléchargement est incomplet, que le fichier a été modifié en route, ou que `checksums.txt` provient d’une autre version, car les hachages sont propres à chaque version.

Téléchargez de nouveau le paquet et `checksums.txt` depuis la même page de version (depuis le miroir si vous êtes en Chine continentale) et vérifiez encore ; si cela ne correspond toujours pas, ne l’exécutez pas. Remplacez `VERSION` et le nom du fichier par ceux de votre téléchargement. Sous Linux :

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict -
```

Sous macOS, remplacez `sha256sum --check --strict -` dans la dernière ligne par `shasum -a 256 -c -`. Dans Windows PowerShell :

```powershell
$file = "obituary_VERSION_windows_amd64.zip"
(Get-FileHash $file -Algorithm SHA256).Hash -eq ((Select-String -Path checksums.txt -SimpleMatch $file).Line -split '\s+')[0]
```

Quand les hachages correspondent, Linux et macOS affichent le nom du fichier suivi de `OK`, et PowerShell affiche `True`.

## Windows affiche « Windows a protégé votre PC »

Windows SmartScreen peut arrêter une archive que vous avez téléchargée à la main dans le navigateur, parce que les programmes d’Obituary ne sont pas encore signés. Avec Scoop, winget ou le script d’installation, cela se produit rarement. Vérifiez d’abord le SHA-256 comme indiqué plus haut ; s’il correspond, choisissez « Informations complémentaires », puis « Exécuter quand même ». Si vous ne pouvez pas le vérifier, ou si Windows signale un logiciel malveillant et pas seulement un éditeur inconnu, ne l’exécutez pas.

## La fenêtre Windows ne s’ouvre pas

Un clic sur Obituary dans le menu Démarrer ou dans la zone de notification ne fait rien, ou un message vous dit qu’un composant d’exécution manque. La fenêtre est dessinée par le runtime Microsoft Edge WebView2. Windows 11 l’inclut et la plupart des machines sous Windows 10 l’ont déjà, mais là où il manque, la fenêtre ne peut pas démarrer. Téléchargez et installez le runtime WebView2 depuis Microsoft (la version Evergreen), puis cliquez de nouveau sur Obituary ; quand la fenêtre s’ouvre, c’est réglé.

La fenêtre n’existe pour l’instant qu’en x64. L’archive Windows pour ARM64 ne contient pas `obit-gui.exe` ; il faut donc y utiliser la ligne de commande. Si « Open Obituary » dans le menu de l’icône affiche une notification disant que le programme de la fenêtre est introuvable, c’est que `obit-gui.exe` n’est pas à côté de `obitd.exe`, ce qui est justement la composition de l’archive ARM64. En x64, placez les deux dans le même dossier.

## macOS : l’app indique qu’elle n’est pas compatible avec cette version

La fenêtre macOS de la 1.6.0 (`Obituary.app` dans le DMG) nécessite macOS 26. Sous macOS 15 et antérieur, l’ouvrir affiche un message disant que l’app n’est pas compatible avec cette version de macOS, et elle ne démarre pas. C’est une erreur de conditionnement de la 1.6.0, corrigée dans la 1.6.1. D’ici là, la fenêtre nécessite macOS 26. Le problème ne concerne que la fenêtre contenue dans le DMG. Une fois la 1.6.1 ou une version ultérieure installée, l’app s’ouvre, et vous le saurez.

## macOS : « impossible de vérifier le développeur »

L’app du DMG n’a pas de signature Developer ID et n’est pas notarisée ; Gatekeeper l’arrête donc la première fois. Après avoir vérifié le SHA-256, ouvrez-la normalement une fois. Quand macOS dit qu’il ne peut pas vérifier le développeur, allez dans Réglages Système, puis Confidentialité et sécurité, trouvez Obituary en bas de la page, choisissez « Ouvrir quand même » et confirmez de nouveau. Les étapes sont dans [Première ouverture d’une app non signée](./desktop#premiere-ouverture-d-une-app-non-signee). Ne désactivez pas Gatekeeper globalement pour cela. Si macOS dit que l’app est endommagée ou contient un logiciel malveillant, c’est autre chose, et il ne faut pas l’autoriser.

## L’icône ne démarre pas à l’ouverture de session (macOS, Ubuntu)

Après l’ouverture de session, il n’y a pas d’icône Obituary dans la barre des menus ou la zone de notification. Le démarrage à l’ouverture de session repose sur une entrée de démarrage : sous macOS, le LaunchAgent utilisateur `site.woodfish.obituary.tray.supervised` ; sous Ubuntu, `obituary-tray.service`. Si l’interrupteur des Settings de la fenêtre est désactivé, si l’app a été déplacée, ou si l’interrupteur a été activé pendant que l’app tournait directement depuis le DMG, le chemin enregistré dans l’entrée est périmé et rien n’est trouvé à l’ouverture de session.

Ouvrez la fenêtre et, dans Settings, désactivez puis réactivez « Start the tray at sign-in » pour qu’Obituary réécrive l’entrée. Sous macOS, `launchctl list | grep obituary` doit maintenant afficher `site.woodfish.obituary.tray.supervised` ; sous Ubuntu, `systemctl --user status obituary-tray.service` indique l’état du service.

## Ubuntu : la fenêtre ne s’ouvre pas, ou il n’y a pas d’icône de notification

Un clic sur Obituary dans la liste des applications ne fait rien, ce qui signifie en général que GTK 3 ou WebKitGTK 4.1 manque. `apt install ./….deb` les installe pour vous ; ils ne manquent que s’ils ont été supprimés ensuite ou si l’installation s’est interrompue en cours de route. Sous Ubuntu 24.04, le paquet GTK 3 s’appelle `libgtk-3-0t64` ; le nom sans `t64`, `libgtk-3-0`, n’existe pas dans cette version, de sorte qu’une commande copiée d’un ancien guide échoue :

```bash
sudo apt update
sudo apt install libgtk-3-0t64 libwebkit2gtk-4.1-0 xdg-utils
```

Rouvrez Obituary depuis la liste des applications ; quand la fenêtre apparaît, c’est réglé. Si la fenêtre s’ouvre mais qu’il n’y a pas d’icône dans la zone de notification, c’est que l’icône nécessite un bureau qui fournisse un hôte StatusNotifierItem, ce que GNOME ne fait pas par défaut. Vous pouvez ouvrir la même fenêtre depuis la liste des applications sans aucune extension ; pour avoir l’icône, ajoutez à GNOME une extension qui la prend en charge, comme AppIndicator.

## Ce qui a expiré n’est pas enterré

Consultez `obit daemon status` pour vérifier la tâche. Utilisez `obit reap` pour traiter immédiatement les éléments arrivés à échéance.

Avec la supervision du bureau activée, le processus résident traite les échéances à temps et une tâche indépendante vérifie les retards toutes les heures. Sans supervision de l’icône pour ce répertoire de données, la tâche s’exécute toutes les cinq minutes.

`obit doctor` indique qui supervise le processus, les échecs récents et le motif du dernier arrêt. Il distingue l’attente avant redémarrage, la pause, l’arrêt volontaire et l’absence ou l’arrêt du superviseur.

Lancez `obit daemon enable` pour installer la tâche (sous Windows, cela installe aussi l’icône au démarrage de session), et `obit sweep` si vous voulez le nettoyage tout de suite. Sur un Linux sans service utilisateur systemd, comme dans certains conteneurs et systèmes minimaux, `obit daemon enable` ne peut pas l’installer ; utilisez cron pour exécuter `obitd --once --home <répertoire de données> --config-dir <répertoire de configuration>` toutes les 5 minutes, et `obit config` vous donne les deux répertoires. Quand la ligne `runner` de `obit daemon status` n’indique plus `off`, c’est réglé ; sous Linux, vous pouvez aussi voir le minuteur avec `systemctl --user list-timers | grep obituary`.

## Un élément a expiré mais n’a pas été enterré

L’élément apparaît dans la page « For you » de la fenêtre, sous « Burials that failed », avec une raison à côté. Il arrive qu’un fichier soit ouvert dans un éditeur, un client de synchronisation ou un autre programme, et il est toujours là. Enterrer, c’est déplacer le fichier, et cela échoue quand il est utilisé ou que les droits manquent. Obituary ne force rien : l’original reste intact, il réessaie après un moment en attendant de plus en plus longtemps, et après plusieurs échecs il s’arrête et attend votre décision.

Fermez ce qui retient le fichier et cliquez sur « Retry », ou lancez `obit sweep`. Si vous ne voulez plus qu’il s’en occupe, cliquez sur « Stop tracking » : le fichier reste où il est et le compte à rebours prend fin. Quand l’élément disparaît de « Burials that failed » et apparaît dans `obit graveyard`, il a été enterré.

## Un fichier ressuscité porte un autre nom

`obit resurrect` n’a signalé aucune erreur, mais le fichier est revenu sous le nom `report (resurrected).txt`, parce qu’un élément portait déjà ce nom à l’emplacement d’origine. La résurrection n’écrase jamais : quand le nom est pris, elle remet le fichier dans le même dossier sous un autre nom, et les deux copies sont intactes. Comparez-les, gardez celle que vous voulez et renommez-la.

## Le fichier de configuration contient une erreur

`obit config` affiche `config file has errors`, suivi d’un chemin de fichier, d’un numéro de ligne et d’un nom de clé. `config.toml` contient une clé qu’Obituary ne connaît pas, ou une valeur non autorisée. Obituary ne l’ignore pas en silence, car `retention = 30` laisserait alors en vigueur la rétention de 7 jours alors que vous croiriez qu’elle a changé. Corrigez l’orthographe, les guillemets ou la durée sur la ligne indiquée et relancez `obit config` ; quand il ne se plaint plus et que la ligne `config` de `obit doctor` est en `PASS`, c’est réglé. La façon d’écrire chaque réglage est décrite dans la [référence de configuration](./config).

## Un hook n’a aucun effet

Après `obit init`, un agent a pu supprimer un fichier sans être arrêté, ou rien ne semble se passer. Il y a trois causes courantes : l’agent n’a pas été redémarré et utilise encore son ancienne configuration ; l’intégration de cet agent a été installée dans un autre périmètre (global ou projet) ; ou `OBIT_DISABLED=1` (désactive tous les hooks) ou `OBIT_GUARD_RM=0` (désactive la garde de suppression) est défini dans l’environnement de l’agent.

Regardez d’abord ce qui est installé :

```bash
obit init -g --show        # ce qui est installé globalement
obit init --show           # ce qui est installé dans le projet courant
obit doctor --verbose      # comment chaque agent est connecté
```

Si ce n’est pas installé, installez-le avec `obit init -g --agent <nom>`, en ajoutant `--dry-run` si vous voulez voir le plan d’abord. Redémarrez ensuite l’agent ; dans Codex, lancez aussi `/hooks` et faites confiance aux hooks d’Obituary. Pour voir ce que le hook reçoit réellement, définissez `OBIT_HOOK_AUDIT=1` dans le terminal d’où vous lancez l’agent, utilisez l’agent un moment, puis lancez `obit hook audit`. Les commandes de suppression refusées n’ont pas besoin de cet interrupteur, car elles sont toujours consignées. Les erreurs du hook lui-même vont dans `hook.log`, dans le répertoire de données, et la ligne `hook-log` de `obit doctor` compte les erreurs des dernières 24 heures.

Quand la ligne de l’agent dans `obit doctor` (par exemple `claude`) est en `PASS` et indique `integration is current`, il est installé ; si vous utilisez la garde de suppression, une ligne comme `claude-guard-rm` doit indiquer `registered: yes`. Lancez `obit doctor` dans le terminal d’où vous démarrez l’agent : si une variable d’environnement a désactivé les hooks, cette ligne passe en `WARN` et nomme la variable.

## La proposition d’un conteneur Docker ne peut pas être approuvée

Dans la fenêtre, « Approve » est grisé sur la proposition d’un conteneur, et la carte indique « Docker is not running: start Docker to decide on this proposal. ». Enregistrer un conteneur revient à demander à Docker son identifiant complet et sa date de création, ce qui est impossible tant que Docker est arrêté. Démarrez Docker Desktop ou Docker Engine et approuvez de nouveau ; quand on peut cliquer sur « Approve » et que `obit docker list` affiche le conteneur, c’est bon.

Quand un conteneur expire, Obituary se contente de l’arrêter et conserve sa couche inscriptible, et le ressusciter le redémarre ; un conteneur arrêté n’est supprimé qu’après la période de rétention.

## La mise au cimetière d’un volume Docker n’est pas confirmée

Les événements Docker sont vérifiés avant et après la suppression. Si celle-ci ne peut pas être confirmée, gardez l’archive vérifiée et consultez `obit doctor` ; ne considérez pas le volume comme enterré. Chaque avertissement `docker-volume-recovery` signale une entrée à vérifier. Suivez les indications pour examiner le volume sur le démon Docker d’origine. Ni `obit resurrect` ni `obit purge` ne prennent en charge une archive non inscrite au registre. [Conteneurs et volumes Docker](./docker)

## Une mise à jour échoue, ou les téléchargements depuis la Chine continentale sont lents

`obit update --check` signale une erreur, ou le script d’installation est lent ou échoue ; le plus probable est que cet ordinateur n’atteint pas GitHub. Quand GitHub tarde à répondre, `obit update --check` et le script d’installation basculent d’eux-mêmes sur le miroir de Chine continentale. Pour utiliser le miroir dès le départ, lancez `bash install.sh --mirror cn` sous macOS et Linux, ou `.\install.ps1 -Mirror cn` sous Windows. Si vous téléchargez à la main, prenez l’archive et `checksums.txt` dans la même version et la même source. Quand `obit update --check` affiche une ligne `Latest:`, c’est réglé.

## Un fichier expiré reste en place

Fermez les programmes qui utilisent le fichier. Si seules les informations sur les processus sont incomplètes, vous pouvez approuver ce fichier en mode `propose`. Le mode automatique doit pouvoir examiner tous les processus du propriétaire du fichier. Il ne peut ignorer les processus inaccessibles d’autres utilisateurs, y compris root, que si le fichier vous appartient et que ni le groupe ni les autres ne peuvent y écrire. Choisissez `propose` pour examiner chaque fichier. L’approbation accepte les lacunes de la liste des processus. Un fichier ouvert reste en place et le motif est affiché. `obit doctor --verbose` · `obit watch --list`
