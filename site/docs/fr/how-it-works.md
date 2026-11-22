# Fonctionnement d’Obituary

## La vie d'un fichier

1. **Enregistrer** : attribuez une durée de vie à un fichier ou à un dossier. L’agent utilise `obit add debug.log 2h` ; dans l’Explorateur, choisissez « Add to Obituary » dans le menu contextuel. Voir [Éléments enregistrés](./auto-register).
2. **Expirer** : `obit list` affiche le temps restant. Avant l’échéance, `obit extend` prolonge la durée de vie et `obit spare` arrête le suivi sans supprimer le fichier.
3. **Enterrer** : à l’échéance, l’élément est transféré au cimetière (`obit graveyard`) et y reste sept jours par défaut.
4. **Ressusciter ou supprimer définitivement** : pendant la conservation, `obit resurrect <id>` remet l’élément à sa place sans jamais écraser de fichier. Ce n’est qu’après ce délai qu’il est supprimé définitivement (`obit purge`).

Vous ne savez pas si un élément vous servira encore ? Enterrez-le avec `obit bury`. Vous pourrez le récupérer tel quel pendant sept jours.

## Où se trouve le cimetière ?

Le cimetière est sur le même disque que l’élément. Sur le disque du répertoire de données, on utilise le cimetière de ce répertoire ; ailleurs, c’est le répertoire caché `.obituary` à la racine du disque. L’enterrement est donc un simple renommage : rapide et sans risque de déplacement partiel.

La durée de conservation se règle dans la configuration. Vous pouvez aussi envoyer les éléments directement à la corbeille du système. Dans ce cas, restaurez-les depuis la corbeille : `obit resurrect` ne peut pas le faire.

```toml
[graveyard]
mode = "graveyard"      # or "recycle": straight to the system Recycle Bin
retention_days = 7      # days the graveyard keeps things
```

Le fichier de configuration est `%APPDATA%\obituary\config.toml`. `obit config` affiche les réglages ; `obit config --create` écrit un fichier commenté avec les valeurs par défaut.

## Pourquoi aucun autre fichier ne peut être supprimé

- **Seule la purge supprime définitivement.** Chaque enterrement crée un dossier de tombe distinct et enregistre son identifiant de fichier. Avant toute suppression définitive ou résurrection, Obituary vérifie que le dossier est toujours le même, qu’il contient exactement un élément et que celui-ci est bien l’objet enterré. Un registre endommagé ou un cimetière remplacé par un lien ne peut donc entraîner aucune suppression hors du cimetière.
- **La résurrection se fait uniquement à l’emplacement d’origine.** Si le nom est pris, rien n’est écrasé : l’élément est restauré à côté sous un nom tel que `name (resurrected).ext`. Les deux sont conservés et le compte rendu indique le chemin réel. Un dossier parent supprimé est recréé. Si un parent est devenu une jonction ou un lien symbolique, si la cible de la jonction a changé ou si la lettre du lecteur correspond à un autre volume, la résurrection est refusée et la tombe reste intacte.
- **Le fichier compte, pas son nom.** Chaque entrée conserve le numéro de série du volume et l’identifiant de fichier sur 128 bits. Obituary retrouve les fichiers renommés ou déplacés et ne touche pas à un autre fichier qui aurait pris leur place. L’identité est vérifiée à nouveau juste avant la suppression. Un fichier ayant plusieurs liens physiques est refusé.
- **Chemins protégés.** Les racines des lecteurs, `C:\Windows`, Program Files, ProgramData, le profil utilisateur, le Bureau, Documents, Téléchargements et les dossiers similaires sont protégés, de même que leurs parents, y compris en cas de redirection. On peut attribuer une durée de vie à leur contenu, pas aux dossiers eux-mêmes.
- **Dépôts Git.** Pour un dossier contenant `.git` à quelque profondeur que ce soit, `obit add` et `obit bury` exigent `--force`. La vérification est répétée à l’échéance. Si elle ne peut aboutir, l’enterrement est refusé.
- **En cas de doute, pas de suppression.** Une entrée n’est abandonnée que si la disparition du fichier est confirmée. Un refus d’accès ou un volume débranché déclenche un nouvel essai ultérieur. L’intervalle commence à une minute et augmente jusqu’à un jour. Après 12 échecs consécutifs, Obituary attend votre décision : le motif figure dans `obit list` et sous « Burials that failed » sur la page For you. `obit extend <id> 1m` (« Retry ») relance l’opération ; `obit spare <id>` (« Stop tracking ») laisse l’élément en place. Avant d’enterrer un gros dossier, Obituary y cherche des dépôts Git pendant 30 secondes au maximum. Si la recherche n’est pas terminée, il réessaie plus tard. Il n’enterre le dossier que si la recherche est complète et qu’il n’y a aucun dépôt.
- **Toutes les opérations sont consignées.** Ajouts, prolongations, annulations et suppressions figurent dans le journal d’audit (`obit log`).

## Fonctionnement en arrière-plan

Avec la supervision du bureau activée, le processus résident traite les échéances à temps et une tâche indépendante vérifie les retards toutes les heures. Sans supervision de l’icône pour ce répertoire de données, la tâche s’exécute toutes les cinq minutes.

Windows utilise une tâche à l’ouverture de session, macOS un LaunchAgent utilisateur et Linux un service systemd utilisateur dans la session graphique. Après un arrêt anormal, le redémarrage a lieu au bout de 60 secondes ; au troisième en 15 minutes, les tentatives cessent.

Les hooks lancent aussi un passage en début et en fin de session. `obit reap` en lance un immédiatement.

```powershell
obit daemon enable              # tray at login plus the fallback scheduled task (--dry-run lists what would be installed)
obit daemon status              # current mode and the next expiry (--json)
obit daemon disable             # remove the scheduled task and login start, and stop the running daemon (--dry-run previews)
```

Les commandes de consultation telles que `list`, `graveyard`, `stats` et `log` sont en lecture seule et n’enterrent rien.

## Emplacement des données {#where-the-data-lives}

Dans `%LOCALAPPDATA%\obituary` se trouvent `store.json` (registre), `audit.jsonl` (journal d’audit), `obitd.log` (journal du démon) et `graveyard\` (cimetière de ce disque).

Vous pouvez modifier ces emplacements avec `OBIT_HOME` (données), `OBIT_CONFIG_DIR` (configuration) et `OBIT_USER_HOME` (répertoire personnel).
