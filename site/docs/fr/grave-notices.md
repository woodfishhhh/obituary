# Avis du cimetière

Enterrer un fichier plutôt que le supprimer n’est utile que si on peut le retrouver. Lorsqu’un outil de l’agent accède à un chemin enterré encore récupérable, ou à un élément d’un dossier enterré, Obituary lui indique :

- le numéro de la tombe ;
- la date de l’enterrement ;
- le temps restant pour ressusciter l’élément ;
- `obit resurrect <id>`.

## Une notification, sans blocage

- L’appel de l’outil n’est ni bloqué ni modifié.
- Chaque tombe est mentionnée une fois par session.
- Claude Code et Codex reçoivent l’avis avant l’exécution de l’outil. Pour les autres agents, le moment de la notification est indiqué par `obit init --show`.
- Le reste du temps, l’agent n’est pas interrompu. Au démarrage, il lui suffit de savoir qu’Obituary est disponible, sans avoir à lire de liste.

Si Obituary est déjà installé, relancez `obit init` pour bénéficier des avis du cimetière.

## Résumé de la session (facultatif)

Pour économiser les jetons de l’agent, le début de session ne signale que les propositions qui attendent votre décision et les éléments qui viennent d’être enregistrés automatiquement. Pour y ajouter un résumé, activez « Session summary » dans les paramètres de la petite fenêtre ou utilisez ce réglage :

```toml
[hooks]
session_summary = true
```

L’agent reçoit alors aussi la liste des éléments du projet qui expirent dans les 24 heures et de ceux récemment enterrés qu’il peut encore ressusciter. Les répertoires temporaires de `obit mktemp` en sont exclus.

## Consulter le cimetière

```powershell
obit graveyard           # what is in the graveyard and when it will be purged
obit graveyard --here    # this folder only
obit resurrect <id>      # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last    # undo the last deletion: everything one obit bury / obit rm buried comes back
```
