# Fenêtre, zone de notification et menu contextuel

Ressuscitez des fichiers, approuvez les propositions et modifiez les réglages sans passer par la ligne de commande.

## Petite fenêtre et icône de barre des menus sur macOS

Téléchargez le DMG universel de macOS et le fichier de checksums de la même version. Vérifiez le SHA-256, ouvrez le DMG puis faites glisser Obituary.app vers Applications ou le dossier Applications de l’utilisateur; éjectez le DMG avant d’activer le lancement à la connexion.

L’app contient la petite fenêtre Wails, l’icône de barre des menus et les auxiliaires obit et obitd. L’agent utilisateur launchd qui vérifie les échéances toutes les 5 minutes reste un secours séparé. Dans les réglages, activez le lancement du tray à la connexion pour installer le LaunchAgent d’Obituary; le désactiver retire uniquement cette entrée.

### Première ouverture d’une app non signée

Cette version n’est ni signée Developer ID ni notariée. Vérifiez la source et le SHA-256, puis essayez de l’ouvrir une fois. Si macOS ne peut pas vérifier le développeur, ouvrez « Réglages Système → Confidentialité et sécurité », choisissez « Ouvrir quand même » pour Obituary, confirmez, puis réessayez. Ne désactivez pas Gatekeeper globalement et ne supprimez pas la quarantaine récursivement. Arrêtez-vous si macOS signale une app endommagée, un logiciel malveillant ou une politique d’organisation.

### Proposition de Homebrew Cask

Le cask doit installer le même DMG universel, copier Obituary.app dans Applications et exposer obit et obitd depuis l’app bundle. Tant que le DMG de bureau n’a pas une URL stable et une vérification de désinstallation relue, il reste séparé du cask CLI existant.
## L’icône de notification

`obit init` affiche une icône dans la zone de notification à l’ouverture de session. Son menu permet de :

- voir les derniers éléments enterrés et cliquer sur l’un d’eux pour le ressusciter ;
- consulter les éléments qui expirent dans les 24 heures ;
- suspendre ou reprendre le nettoyage automatique ;
- choisir « Open Obituary » pour ouvrir la petite fenêtre ; une notification explique le problème si `obit-gui.exe` n’est pas à côté de `obitd.exe` ;
- quitter le programme de la zone de notification sans arrêter la tâche planifiée.

## La petite fenêtre

`obit-gui.exe` comporte cinq pages : Upcoming, Graveyard, Decisions, Settings et Logs. Le raccourci « Obituary » du menu Démarrer, celui du Bureau et « Open Obituary » dans la zone de notification ouvrent tous la même fenêtre.

- La barre en haut de l’accueil indique « Auto registration: off » ou « on ». Cliquez dessus pour accéder aux réglages. En haut de Decisions ou dans Settings, choisissez entre désactivé (par défaut, seuls les éléments que vous ou l’agent enregistrez sont pris en compte), par niveaux, entièrement automatique ou toujours demander. Voir [Éléments enregistrés](./auto-register). Plus bas, « Handled automatically » liste les enregistrements sans demande préalable. Un clic suffit pour les annuler (le décompte s’arrête et l’élément reste en place) ou prolonger leur durée de vie.
- Un fichier arrivé à échéance mais impossible à enterrer, par exemple parce qu’un autre programme le garde ouvert, apparaît dans « Burials that failed » sur la page Decisions, avec le motif et l’heure du prochain essai. Obituary espace progressivement ses tentatives. Si les échecs se répètent, il attend votre décision : « Retry » réessaie immédiatement, « Stop tracking » arrête le suivi et laisse l’élément en place.
- Si Docker ne tourne pas, le bouton d’approbation d’une proposition de conteneur est désactivé et la carte en indique la raison. Approuvez-la une fois Docker démarré.
- La confiance accordée à un fichier `.obitrules` approuve exactement la version affichée dans la fenêtre.
- La langue de la fenêtre, de la zone de notification et du menu contextuel suit `[ui] language` dans `config.toml` (`auto`, `zh-CN` ou `en` ; `auto` reprend la langue de Windows). Vous pouvez aussi la changer dans les réglages.
- La fenêtre est réservée à x64 et nécessite le runtime WebView2, intégré à Windows 11 et présent sur la plupart des PC sous Windows 10.

### Mises à jour

Pour installer une version stable plus récente, ouvrez Settings → About and updates. Obituary ferme la fenêtre et le programme de la zone de notification, effectue la mise à jour par le canal d’installation d’origine (Scoop, WinGet ou script), puis tente de rétablir l’icône et de rouvrir la fenêtre. La même page indique les échecs et le chemin des journaux. Si vous avez extrait le programme vous-même, téléchargez la nouvelle copie sur la page des versions.

### Huit apparences

Choisissez une apparence dans Settings → Appearance. Le changement est immédiat et ne concerne que la fenêtre de cet ordinateur.

| Apparence | Description |
|---|---|
| Calendrier | Une colonne par jour. Chaque fichier apparaît à sa date d’enterrement ou de suppression définitive. Faites-le glisser vers un autre jour pour prolonger sa durée de vie. |
| Une ligne | Une seule ligne de saisie, tout au clavier : cherchez un fichier et appuyez sur Entrée pour le ressusciter ou prolonger sa durée de vie. |
| Tunnel | Le temps est représenté par un tunnel en 3D : plus le fichier est proche, plus son échéance est imminente. Faites défiler pour parcourir les dates. |
| Veille | Fond sombre avec une lampe par fichier. Plus elle est faible, plus l’échéance est proche. |
| Avis | Une page de nécrologie en colonnes avec une police à empattements ; un tampon apparaît à la résurrection. |
| Horizon | Les fichiers suivis sont au-dessus du sol, le cimetière en dessous. |
| Mica | L’aspect natif de Windows 11 avec une barre latérale. |
| Nuit | Un thème sombre et discret avec une barre supérieure. |

## Menu contextuel

`obit init` installe par défaut le menu contextuel de l’Explorateur pour l’utilisateur courant uniquement. Faites un clic droit sur un fichier ou un dossier, puis choisissez « Add to Obituary » :

- **1 jour / 7 jours / 30 jours** : l’élément est enterré à l’échéance ; vous pouvez l’utiliser normalement jusque-là.
- **Bury now (recoverable)** : l’élément part immédiatement au cimetière et reste récupérable pendant le délai de conservation (sept jours par défaut).

Vous pouvez ajouter plusieurs fichiers en une fois. Une notification indique le nombre d’ajouts réussis, la date d’enterrement et le nombre d’échecs. Le nouveau menu de Windows 11 masque ces entrées : choisissez « Afficher plus d’options » ou maintenez Maj en faisant un clic droit. `obit init --dry-run` liste les entrées et leurs commandes ; `obit doctor` vérifie l’installation.

macOS et Linux n’ont pas encore de menu contextuel. Utilisez `obit mark <path> 7d` pour ajouter un élément ou `obit bury <path>` pour l’enterrer immédiatement.

## Icônes et menus

`obit init` configure ces éléments par défaut. Chacun possède un réglage dans Settings → Icons and menus :

| Réglage | Effet |
|---|---|
| Icône de notification | Icône Obituary dans la zone de notification. Le nettoyage continue même si elle est masquée. |
| Icône de la barre des tâches | Détermine si la fenêtre occupe une place dans la barre des tâches lorsqu’elle est ouverte. |
| Menu Démarrer | Entrée « Obituary » dans Démarrer. |
| Raccourci du bureau | "Obituary" sur le bureau |
| Menu contextuel | Entrée « Add to Obituary » dans l’Explorateur. |

À l’installation, `obit init --no-context-menu` omet le menu contextuel et `--no-shortcuts` les raccourcis du menu Démarrer et du Bureau.

Les dates d’enterrement affichées dans l’Explorateur sont expliquées dans [Règles de dossier](./folder-policies#dates-in-explorer).
## Comparaison des quatre plateformes

| Plateforme | Fenêtre | Barre système ou menus | Connexion et notifications | Menu contextuel et dates | Architecture et dépendance | Après désinstallation |
| --- | --- | --- | --- | --- | --- | --- |
| Windows | `obit-gui.exe` | Zone de notification | Tâche planifiée et démarrage facultatif | Menu de l’Explorateur et dates | x64, WebView2 requis | Programmes et intégrations retirés, données conservées |
| Aperçu macOS | `Obituary.app` universelle | Barre des menus | LaunchAgent et notifications facultatifs | Aucun menu contextuel ni date Finder | Intel et Apple, app non signée/non notariée | app et LaunchAgent retirés, données CLI conservées |
| Aperçu Ubuntu 24.04 | Fenêtre du `.deb` | Barre StatusNotifierItem | XDG autostart et notifications facultatifs | Aucun menu contextuel ni date | amd64, GTK 3 et WebKitGTK 4.1 | Paquet et autostart retirés, données conservées |
| macOS/Linux CLI | Aucune | Aucune | launchd, systemd ou crontab | Aucun menu ni date | CLI x64 et ARM64 | Programmes et services retirés, données conservées |

Vérifiez SHA-256 avant d’utiliser un paquet non signé ou en aperçu. Pour macOS, suivez la [procédure de première ouverture](#première-ouverture-dune-app-non-signée). Le mode recycle ne permet pas `resurrect`.
