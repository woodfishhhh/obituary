# Fenêtre, zone de notification et menu contextuel

Ressuscitez des fichiers, approuvez les propositions et modifiez les réglages sans passer par la ligne de commande.

## Ce que propose chacune des trois plateformes

La fenêtre, l’icône de notification et les notifications de bureau existent sous Windows, macOS et Ubuntu 24.04 ; le menu contextuel et les étiquettes de date de l’Explorateur n’existent que sous Windows. Les éditions de bureau pour macOS et Ubuntu sont pour l’instant des préversions. Les autres distributions Linux n’ont pas d’édition de bureau et utilisent la ligne de commande.

| | Windows | macOS | Ubuntu 24.04 |
| --- | --- | --- | --- |
| Fenêtre | `obit-gui.exe`, x64 uniquement | `Obituary.app`, universelle pour Intel et Apple Silicon | un paquet `.deb`, amd64 uniquement |
| Icône | zone de notification | barre des menus | icône de notification ; nécessite un bureau qui fournit StatusNotifierItem |
| Icône à l’ouverture de session | activée par défaut après `obit init` | interrupteur dans Settings | interrupteur dans Settings |
| Vérification des expirations | processus résident de l’icône | LaunchAgent | systemd |
| Notifications de bureau | oui | oui | oui |
| Menu contextuel | dans l’Explorateur | non | non |
| Étiquettes de date dans l’Explorateur | oui | non | non |
| Mode Corbeille | oui | non, cimetière seulement | oui |
| Installé avec | le script d’installation, Scoop, winget ou l’archive | le DMG | le `.deb` |
| Première ouverture | une archive téléchargée à la main peut déclencher un avertissement SmartScreen | demande une autorisation manuelle, voir [plus bas](#premiere-ouverture-d-une-app-non-signee) | s’ouvre directement |
| Prérequis | le runtime WebView2 | la 1.6.0 nécessite macOS 26 (corrigé dans la 1.6.1) | GTK 3 et WebKitGTK 4.1, installés par apt |

Windows utilise une tâche à l’ouverture de session, macOS un LaunchAgent utilisateur et Linux un service systemd utilisateur dans la session graphique. Après un arrêt anormal, le redémarrage a lieu au bout de 60 secondes ; au troisième en 15 minutes, les tentatives cessent.

Avec la supervision du bureau activée, le processus résident traite les échéances à temps et une tâche indépendante vérifie les retards toutes les heures. Sans supervision de l’icône pour ce répertoire de données, la tâche s’exécute toutes les cinq minutes.

Sous Windows, seules les anciennes tâches conformes à la définition créée pour cette installation sont migrées. La définition d’origine est conservée pour le retour en arrière ; les tâches modifiées ou désactivées restent intactes.

## Petite fenêtre et icône de barre des menus sur macOS

Téléchargez le DMG universel (`obituary_<version>_darwin_universal.dmg`) et `checksums.txt` depuis la page de la version, vérifiez le SHA-256, ouvrez le DMG, faites glisser Obituary.app dans Applications, éjectez le DMG, puis ouvrez l’app depuis Applications. Faites-la glisser d’abord, parce que l’entrée de démarrage à l’ouverture de session enregistre l’emplacement de l’app, et une app lancée directement depuis le DMG n’y est plus une fois le DMG éjecté.

L’app contient la fenêtre, l’icône de la barre des menus et les programmes `obit` et `obitd` qui l’accompagnent. Activer « Start the tray at sign-in » dans les Settings de la fenêtre installe le LaunchAgent utilisateur propre à Obituary, pour que l’icône de la barre des menus apparaisse à l’ouverture de session ; le désactiver supprime uniquement cette entrée.

Installez la tâche indépendante de vérification des échéances avec `obit daemon enable`. Fermer la fenêtre ou quitter l’icône de la barre des menus ne la désactive pas.

Supprimer `Obituary.app` laisse vos données dans `~/Library/Application Support/obituary/` par défaut : `store.json` contient le registre, `config.toml` les réglages et `graveyard/` les fichiers enterrés sur ce volume.

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

Pour installer une version stable plus récente, ouvrez Settings → About and updates. Obituary ferme la fenêtre et le programme de la zone de notification, effectue la mise à jour par le canal d’installation d’origine (Scoop, WinGet ou script), puis tente de rétablir l’icône et de rouvrir la fenêtre. La même page indique les échecs et le chemin des journaux.

Une copie que vous avez extraite vous-même se met aussi à jour sur place ; son bouton affiche « Download and restart to update ». Obituary télécharge la version depuis GitHub (depuis le miroir de Chine continentale si GitHub ne répond pas en dix secondes), vérifie son empreinte SHA-256, puis seulement ferme la fenêtre et remplace les programmes propres à Obituary dans ce dossier (`obit`, `obitd` et `obit-gui`). Les autres fichiers du dossier ne sont pas touchés, ni PATH ni le menu Démarrer. Les anciens programmes restent un jour dans un dossier masqué `.obituary-update-…` au même endroit, puis vont au cimetière, où ils peuvent encore être ressuscités ; si une étape échoue, y compris un nouveau `obit` qui ne démarre pas, la version précédente est remise en place. Il faut une version stable (pas une version de développement), tous les programmes de la version présents dans le dossier comme fichiers ordinaires, et un dossier où l’utilisateur actuel peut écrire sans droits d’administrateur. Si l’une de ces conditions manque, la page indique laquelle et vous pouvez toujours télécharger la nouvelle copie sur la page des versions. Dans un terminal, `obit update --apply` fait la même chose. « What changed » affiche les notes dans la langue de la fenêtre, en anglais s’il n’y a pas de traduction.

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

Après une mise à niveau, si tous les libellés du menu de l’Explorateur sont dans une autre langue prise en charge, le menu reste utilisable. `obit doctor` le signale à titre informatif (JSON : `installation: label-language-mismatch`). `obit init --context-menu --yes` applique la langue actuelle de l’interface.

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

Si vous n’en voulez pas dès l’installation, ajoutez `obit init --no-context-menu` pour omettre le menu contextuel ou `--no-shortcuts` pour omettre les raccourcis du menu Démarrer et du Bureau. Les mêmes options valent à la désinstallation : `obit init --uninstall --no-shortcuts` laisse en place les raccourcis existants et retire le reste comme d’habitude, et `--no-context-menu` fait de même pour le menu contextuel.

Les dates d’enterrement affichées dans l’Explorateur sont expliquées dans [Règles de dossier](./folder-policies#dates-in-explorer).
