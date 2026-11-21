# Fenêtre, zone de notification et menu contextuel

Ressuscitez des fichiers, approuvez les propositions et modifiez les réglages sans passer par la ligne de commande.

## Ce que propose chacune des trois plateformes

La fenêtre, l’icône de notification et les notifications de bureau existent sous Windows, macOS et Ubuntu 24.04 ; le menu contextuel, les étiquettes de date de l’Explorateur et le mode Corbeille n’existent que sous Windows. Les éditions de bureau pour macOS et Ubuntu sont pour l’instant des préversions. Les autres distributions Linux n’ont pas d’édition de bureau et utilisent la ligne de commande.

| | Windows | macOS | Ubuntu 24.04 |
| --- | --- | --- | --- |
| Fenêtre | `obit-gui.exe`, x64 uniquement | `Obituary.app`, universelle pour Intel et Apple Silicon | un paquet `.deb`, amd64 uniquement |
| Icône | zone de notification | barre des menus | icône de notification ; nécessite un bureau qui fournit StatusNotifierItem |
| Icône à l’ouverture de session | activée par défaut après `obit init` | interrupteur dans Settings | interrupteur dans Settings |
| Vérification des expirations | processus résident de l’icône, Planificateur de tâches en secours | agent utilisateur launchd, toutes les 5 minutes | minuteur utilisateur systemd, toutes les 5 minutes |
| Notifications de bureau | oui | oui | oui |
| Menu contextuel | dans l’Explorateur | non | non |
| Étiquettes de date dans l’Explorateur | oui | non | non |
| Mode Corbeille | oui | non, cimetière seulement | non, cimetière seulement |
| Installé avec | le script d’installation, Scoop, winget ou l’archive | le DMG | le `.deb` |
| Première ouverture | une archive téléchargée à la main peut déclencher un avertissement SmartScreen | demande une autorisation manuelle, voir [plus bas](#premiere-ouverture-d-une-app-non-signee) | s’ouvre directement |
| Prérequis | le runtime WebView2 | la 1.6.0 nécessite macOS 26 (corrigé dans la 1.6.1) | GTK 3 et WebKitGTK 4.1, installés par apt |

## Petite fenêtre et icône de barre des menus sur macOS

Téléchargez le DMG universel (`obituary_<version>_darwin_universal.dmg`) et `checksums.txt` depuis la page de la version, vérifiez le SHA-256, ouvrez le DMG, faites glisser Obituary.app dans Applications, éjectez le DMG, puis ouvrez l’app depuis Applications. Faites-la glisser d’abord, parce que l’entrée de démarrage à l’ouverture de session enregistre l’emplacement de l’app, et une app lancée directement depuis le DMG n’y est plus une fois le DMG éjecté.

L’app contient la fenêtre, l’icône de la barre des menus et les programmes `obit` et `obitd` qui l’accompagnent. Les expirations sont gérées par un autre agent utilisateur launchd qui s’exécute toutes les 5 minutes ; vous l’installez avec `obit daemon enable`, et fermer la fenêtre ou l’icône de la barre des menus ne l’affecte pas. Activer « Start the tray at sign-in » dans les Settings de la fenêtre installe le LaunchAgent utilisateur propre à Obituary, pour que l’icône de la barre des menus apparaisse à l’ouverture de session ; le désactiver supprime uniquement cette entrée.

### Première ouverture d’une app non signée

La version n’a pas de signature Developer ID et n’est pas notarisée ; macOS l’arrête donc la première fois. Après vous être assuré que le téléchargement vient d’une page de version de confiance et avoir vérifié le SHA-256, ouvrez-la normalement une fois. Si le système dit qu’il ne peut pas vérifier le développeur, ouvrez Réglages Système, puis Confidentialité et sécurité, trouvez Obituary en bas de la page, choisissez « Ouvrir quand même », confirmez encore une fois et rouvrez l’app. Ne désactivez pas Gatekeeper globalement pour cela et n’effacez pas récursivement l’attribut de quarantaine. Si le système signale que l’app est endommagée ou contient un logiciel malveillant, ou qu’une politique de l’entreprise interdit de l’ouvrir, arrêtez-vous là.

### La 1.6.0 nécessite macOS 26

La fenêtre macOS de la 1.6.0 nécessite macOS 26. Sous macOS 15 et antérieur, le système indique que l’app n’est pas compatible avec cette version, et elle ne s’ouvre pas. C’est corrigé dans la 1.6.1.

## Ubuntu 24.04 (amd64)

Téléchargez le paquet de bureau `obituary_<version>_linux_amd64.deb` et `checksums.txt` depuis la page de la version. Depuis le répertoire de téléchargement, lancez les commandes ci-dessous en remplaçant `VERSION` par la version téléchargée ; le paquet n’est installé que si la vérification réussit :

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict - && sudo apt install "./$file"
```

Ouvrez ensuite Obituary depuis la liste des applications. La fenêtre utilise GTK 3 et WebKitGTK 4.1, et apt les installe avec le paquet. Les programmes se trouvent dans `/usr/libexec/obituary/`, qui n’est pas dans votre PATH ; pour utiliser `obit` dans un terminal, installez la ligne de commande séparément avec le script d’installation.

L’icône de notification nécessite un bureau qui fournisse StatusNotifierItem, ce que GNOME ne fait pas par défaut. Vous y ouvrez la même fenêtre depuis la liste des applications, sans aucune extension, mais il n’y a pas d’icône. Pour que l’icône apparaisse à votre prochaine ouverture de session, activez « Start the tray at sign-in » dans Settings ; le désactiver supprime l’entrée de démarrage qu’Obituary a écrite. Pour la désinstallation, voir [Désinstallation](./uninstall).

## L’icône de notification

`obit init` affiche une icône dans la zone de notification à l’ouverture de session. Son menu permet de :

- voir les derniers éléments enterrés et cliquer sur l’un d’eux pour le ressusciter ;
- consulter les éléments qui expirent dans les 24 heures ;
- suspendre ou reprendre le nettoyage automatique ;
- choisir « Open Obituary » pour ouvrir la petite fenêtre ; une notification explique le problème si `obit-gui.exe` n’est pas à côté de `obitd.exe` ;
- quitter le programme de la zone de notification sans arrêter la tâche planifiée.

## La petite fenêtre

`obit-gui.exe` comporte cinq pages : Expiring, Graveyard, For you, Settings et Logs. Le raccourci « Obituary » du menu Démarrer, celui du Bureau et « Open Obituary » dans la zone de notification ouvrent tous la même fenêtre.

- La barre en haut de l’accueil indique « Auto registration: off » ou « on ». Cliquez dessus pour accéder aux réglages. En haut de For you ou dans Settings, choisissez entre désactivé (par défaut, seuls les éléments que vous ou l’agent enregistrez sont pris en compte), par niveaux, entièrement automatique ou toujours demander. Voir [Éléments enregistrés](./auto-register). Plus bas, « Handled automatically » liste les enregistrements sans demande préalable. Un clic suffit pour les annuler (le décompte s’arrête et l’élément reste en place) ou prolonger leur durée de vie.
- Un fichier arrivé à échéance mais impossible à enterrer, par exemple parce qu’un autre programme le garde ouvert, apparaît dans « Burials that failed » sur la page For you, avec le motif et l’heure du prochain essai. Obituary espace progressivement ses tentatives. Si les échecs se répètent, il attend votre décision : « Retry » réessaie immédiatement, « Stop tracking » arrête le suivi et laisse l’élément en place.
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
