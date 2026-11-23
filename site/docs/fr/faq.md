# Questions fréquentes

## Pourquoi ne pas supprimer directement ?

Si vous supprimez un fichier dont vous aurez finalement besoin, vous ne pourrez pas le récupérer. Obituary l’enterre dans le cimetière et le conserve sept jours par défaut. Pendant ce délai, `obit resurrect` le remet à sa place ; la suppression définitive n’intervient qu’ensuite. Si l’agent cherche le fichier pendant ces sept jours, il reçoit immédiatement le numéro de la tombe et la commande de résurrection (voir les [avis du cimetière](./grave-notices)).

## Obituary fonctionne-t-il sous macOS et Linux ?

Oui. Les deux systèmes disposent d’un outil en ligne de commande et d’un programme d’arrière-plan pour x64 et ARM64, avec cimetière, résurrection et enterrement automatique. Pour vérifier les échéances, macOS utilise launchd et Linux un minuteur systemd de l’utilisateur. Les hooks de l’agent peuvent aussi lancer une vérification si nécessaire. Voir le [démarrage rapide](./index). Sous Windows et Linux, `graveyard.mode = "recycle"` utilise la Corbeille du système. Restaurez les fichiers depuis celle-ci, et non avec `obit resurrect`. Sous macOS, seul le cimetière est disponible.

## Windows indique « Windows a protégé votre ordinateur » ?

Obituary n’est pas encore signé. Si cet avertissement apparaît après le téléchargement de l’archive depuis un navigateur, choisissez « Informations complémentaires → Exécuter quand même ». Scoop, winget et le script d’installation ne le déclenchent généralement pas ; tous trois vérifient d’abord l’empreinte SHA-256 de l’archive.

## La petite fenêtre est-elle disponible sur ARM64 ?

Non. `obit-gui.exe` n’existe que pour x64. La version ARM64 comprend la ligne de commande et l’icône de notification.

## Mon code peut-il être enregistré avant que je l’aie inclus dans un commit ?

Non. L’enregistrement automatique est désactivé par défaut : seuls sont pris en compte les éléments que l’agent enregistre avec `obit mark` et ceux que vous ajoutez depuis le menu contextuel. Même activé, il ne concerne que les éléments non suivis dans une copie de travail Git. Les nouveaux fichiers qu’aucune règle ne couvre (du code non encore commité, par exemple) ne sont jamais listés ; dépendances, sorties de compilation et caches sont laissés intacts. Un élément enregistré par règle ou sur proposition n’est pas enterré s’il figure dans un commit à l’échéance. Voir [Éléments enregistrés](./auto-register).

## Que se passe-t-il si Git échoue ou si mon `.gitignore` couvre beaucoup de fichiers ?

Avant tout enregistrement, Obituary demande à Git s’il suit l’élément. Si Git ne peut répondre (absent, délai dépassé, dépôt endommagé), rien n’est enregistré. Les dossiers hors d’un dépôt Git ne sont jamais enregistrés automatiquement ; seuls vous ou l’agent pouvez leur attribuer une durée de vie avec `obit mark`. Un `.gitignore` très large laisse davantage d’éléments non suivis, mais seules les règles intégrées et le fichier `.obitrules` auquel vous avez accordé votre confiance s’appliquent.

## Un agent peut-il faire passer mes fichiers pour les siens et éviter la proposition ?

Non. Par défaut, l’enregistrement automatique est désactivé et rien n’est enregistré automatiquement sans votre accord. En mode par niveaux, seuls les fichiers dont la création par l’agent pendant sa session est établie passent sans proposition : le hook a observé l’écriture, le fichier était absent de l’instantané initial et sa date de création dans le système de fichiers est postérieure au début de la session. Un fichier qui existait déjà ou qu’un autre programme a créé fait l’objet d’une proposition. Même si l’agent est incité à renommer un ancien dossier pour qu’il corresponde à une règle, sa date de création ne change pas : il sera seulement proposé. Dans tous les modes, les fichiers suivis par Git sont exclus de l’enregistrement automatique. Tout enregistrement direct vous est signalé après coup et peut être annulé par `obit spare` ; les éléments enterrés restent récupérables sept jours.

## Un agent peut-il approuver des propositions pour moi ?

Non. Aucun outil MCP ne permet d’approuver ou de refuser des propositions, de faire confiance à `.obitrules`, de supprimer définitivement des éléments du cimetière ou de changer la configuration, y compris le mode de proposition. L’agent peut seulement vous transmettre la proposition.

## `obit resurrect` fonctionne-t-il en mode corbeille ?

Non. Avec `mode = "recycle"` dans `config.toml`, les éléments enterrés vont dans la corbeille du système. C’est depuis celle-ci qu’il faut les restaurer.

## Comment désactiver temporairement une fonction ?

- Hooks : `OBIT_DISABLED=1`
- Garde de suppression : `OBIT_GUARD_RM=0`
- Nettoyage automatique : choisissez « Pause » dans le menu de l’icône de notification.

## Où sont les données ?

`%LOCALAPPDATA%\obituary` contient le registre, le journal d’audit, le journal du démon et le cimetière de ce disque. La configuration se trouve dans `%APPDATA%\obituary\config.toml`. Voir [Fonctionnement](./how-it-works#where-the-data-lives).

## Est-ce que la désinstallation supprime le cimetière ?

Non. `obit uninstall` conserve le cimetière et le registre par défaut. N’ajoutez `--purge-data` que si vous voulez vraiment les supprimer. Voir [Désinstallation](./uninstall).
