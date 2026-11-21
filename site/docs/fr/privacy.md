# Politique de confidentialité

Le registre, le cimetière, les journaux et les réglages d’Obituary restent sur votre ordinateur. Obituary ne transmet ni rapports de plantage, ni fichiers, ni chemins, ni contenus de fichiers.

## Statistiques d’utilisation

Les statistiques sont activées par défaut. Elles servent seulement à compter les installations actives et à mesurer la répartition des versions. Lors de l’utilisation effective du logiciel, Obituary peut envoyer, au plus une fois par période de 24 heures, un identifiant d’installation aléatoire, la version, le système d’exploitation, l’architecture, la langue de l’interface et le canal d’installation. Aucun fichier, chemin ou commande n’est envoyé ; aucune adresse IP ni aucun en-tête User-Agent n’est enregistré. Exécutez `obit config telemetry off` ou définissez `OBIT_TELEMETRY=0` pour désactiver l’envoi. Aucun rapport n’est envoyé en intégration continue (CI) ou avec `DO_NOT_TRACK=1`.

Le serveur ne conserve ni adresses IP, ni User-Agents, ni identifiants d’installation bruts. Une empreinte calculée avec une clé distincte permet de rattacher plusieurs rapports à une même installation, sans identifier une personne. Les données d’activité quotidiennes expirent après 400 jours. Seules l’empreinte et les dates de première et de dernière observation sont conservées à long terme pour éviter les doublons. Les rapports transmis avec succès sont comptés uniquement sous forme de totaux par canal. Seuls les responsables de la maintenance ont accès au tableau de bord après authentification.

La désactivation efface l’identifiant d’installation local et tout rapport en attente. La recherche de mises à jour a son propre réglage.

## Recherche de nouvelles versions

- Au plus une fois par jour, Obituary lit les informations publiques sur la dernière version du dépôt sur `https://api.github.com`. Si GitHub ne répond pas, il consulte `https://cn.obituary.woodfish.site/download/latest.json` sur le miroir de Chine continentale.
- La requête ne contient que les informations habituelles : votre adresse IP et un User-Agent indiquant la version d’Obituary, par exemple `Obituary/1.0.0`.
- Vous pouvez désactiver la recherche à tout moment : décochez « Check for updates automatically » dans les réglages de la fenêtre ou ajoutez ceci à `%APPDATA%\obituary\config.toml` :

  ```toml
  [update]
  check = false
  ```

  Obituary ne fera alors de vérification que si vous exécutez vous-même `obit update --check`.

## Installation et téléchargements

- Scoop, winget et le script d’installation téléchargent les archives depuis [GitHub Releases](https://github.com/woodfishhhh/obituary/releases). La [déclaration de confidentialité de GitHub](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement) s’applique.
- Avec le miroir (`-Mirror cn`), les archives viennent de `cn.obituary.woodfish.site`. Ce serveur tient des journaux d’accès ordinaires (heure, adresse IP, URL demandée) pour le diagnostic des problèmes.
- Les mises à jour depuis l’application (« Update now » ou « Download and restart to update » dans la fenêtre, et `obit update --apply`) téléchargent l’archive de la version et `checksums.txt` depuis GitHub Releases, ou depuis `cn.obituary.woodfish.site` si GitHub ne répond pas en dix secondes. Les requêtes ne portent que votre adresse IP et un User-Agent indiquant la version d’Obituary, et aucune donnée d’usage n’est envoyée.
- Lors de la publication, GitHub Actions envoie les fichiers au miroir. Celui-ci les vérifie et les distribue localement, sans transmettre votre demande de téléchargement à GitHub.

## Connexion des agents

`obit init` ne modifie que les fichiers de configuration des agents sur votre ordinateur, après vous avoir présenté les changements prévus. Il n’envoie rien aux fournisseurs des agents ni à quiconque.

## Site web

- <https://obituary.woodfish.site/> n’utilise ni outil d’analyse d’audience ni script de suivi. Notre serveur dessert la Chine continentale et GitHub Pages le reste du monde. Tous deux tiennent des journaux d’accès ordinaires.
- Une adresse e-mail laissée sur le site sert uniquement à annoncer les nouvelles plateformes et versions. Elle est conservée sur notre propre serveur et n’est pas communiquée à des tiers. Chaque message contient un lien de désabonnement.

## Contact

Posez vos questions sur la confidentialité dans un [ticket GitHub](https://github.com/woodfishhhh/obituary/issues). Signalez les problèmes de sécurité en privé, comme indiqué dans [SECURITY.md](https://github.com/woodfishhhh/obituary/blob/main/SECURITY.md).
