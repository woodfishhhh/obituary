# Garde de suppression

Effacer un dossier personnel ou un disque entier n’est plus une hypothèse. Lorsque la garde est active, l’exécution de `rm -rf build`, `rmdir`, `git clean -fdx`, `Remove-Item -Recurse` ou d’une suppression comparable est arrêtée et l’agent reçoit une commande équivalente, par exemple `obit rm -rf build`. Une fois cette commande utilisée, les éléments vont au cimetière au lieu de disparaître et `obit resurrect <id>` les restaure.

```powershell
obit init -g --guard-rm                 # Claude Code; add --agent codex for Codex. Off by default
```

## Un cas réel

Garde activée, le hook a reçu un appel Bash de Claude Code, `rm -rf build`, et a répondu comme suit ; `rm` n’a jamais été exécuté :

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Obituary's delete guard is on: `'C:/work/obit.exe' rm -rf build` provides restorable deletion; `obit resurrect` restores graveyard entries."}}
```

L’agent passe alors à Obituary : le dossier est enterré, puis une commande le ressuscite.

```console
$ obit rm -rf build
$ obit graveyard --here
ID  BURIED            PURGE  SIZE  PATH
#2  2026-11-11 00:36  in 7d  23 B  C:\work\build
$ obit resurrect --last
#2 resurrected  C:\work\build
```

## Principes

- **Refuser, sans réécrire.** Obituary ne réécrit pas la commande de l’agent. Claude Code vérifie les autorisations sur la commande réécrite, ce qui pourrait neutraliser une règle `deny` visant la commande initiale. Refuser l’original et laisser l’agent émettre l’équivalent conserve toutes les règles d’autorisation.
- **Toute suppression littérale est arrêtée.** Si `rm`, `rmdir`, `git clean` ou `Remove-Item` (y compris `del`, `rd`, `ri`) apparaît à n’importe quelle position, après `&&`, `||` ou `;`, dans un pipeline, `$(…)`, un sous-shell, une condition ou une boucle, l’exécution est arrêtée. Cela inclut `rm -rf build && npm run build`, `rm -rf build 2>/dev/null` et `git clean -fdx && npm ci`. Un `rm` dans une chaîne, un commentaire ou un argument d’`echo` ne compte pas, pas plus que `rm --help`, `git clean -n` et `Remove-Item -WhatIf`.
- **Lister d’abord les cibles choisies à l’exécution.** `find -delete`, `find -exec rm` (y compris `-execdir`), `xargs rm` et `Get-ChildItem … | Remove-Item` sont refusés sans réécriture. Examinez les correspondances, puis transmettez les chemins explicites à `obit rm -- <path>...`. Un `find` qui ne fait que chercher ou lister des chemins s’exécute. La garde n’inspecte pas les wrappers arbitraires tels que `sudo rm` ou `bash -c '…'`.
- **Aucun retour vers une suppression réelle.** Chaque chemin fourni à `obit rm`, `obit remove-item` ou `obit git-clean` est enterré. En cas de refus, l’erreur suit le format de `rm` et le code de sortie est non nul.
- **Le chemin réel est vérifié.** Entre la commande voulue et celle qui s’exécute se trouvent guillemets, échappements et parfois un proxy. Un caractère perdu peut transformer `rm -rf "$OUT"/` en `rm -rf /`. `obit rm` vérifie donc le chemin réel sur le disque avant d’agir.
- **Sous Windows, Codex examine aussi les substitutions entre guillemets.** Une autre shell peut exécuter du texte que Bash aurait gardé littéral. Chaque `$(…)` est lu selon Bash et PowerShell, dans les guillemets, heredocs cités et commentaires compris. Une suppression arrête l’exécution ; le texte contenant des backticks peut être écrit dans un fichier puis transmis comme chemin.
- Les fichiers suivis par Git sont eux aussi enterrés afin de pouvoir récupérer les changements non commités ; c’est une suppression volontaire de l’agent, différente de l’enregistrement automatique.
- `obit rm -rf` sur un dossier contenant `.git` enterre le dépôt entier. Un vrai `rm -rf` le détruirait, tandis que l’enterrement peut être annulé. Les protections des dossiers système et utilisateur restent actives.
- **La commande équivalente est volontairement étroite.** Elle ne doit jamais sélectionner plus que l’original. Obituary ne traduit donc pas entre shells et ne propose un équivalent que pour les formes du tableau. Sinon, la suppression est bloquée et l’explication indique que `obit bury <path>` et `obit git-clean` permettent une suppression récupérable.

## Ce qui est arrêté et ce qui passe

| Hôte | Arrêt avec commande équivalente | Arrêt avec motif et conseils généraux | Passe |
|---|---|---|---|
| Claude Code (outil Bash) | `rm`, `rmdir`, `git clean`, éventuellement précédés de `cd x` littéral relié par `&&`, `;` ou un saut de ligne ; `rm -f *.log` si aucun fichier ne commence par `-`, `rm -rf ../dist`, `rm -- "$target"` avec la variable après `--` | Toute autre suppression littérale : instructions avant ou après (`npm …`, `trap`, `hash`), variables avant `--`, chemins POSIX absolus commençant par `/`, redirections, `\|\|`, pipelines, `$(…)`, suppressions interactives, `/usr/bin/rm` avec un chemin | Commandes qui ne suppriment rien |
| Codex, sous `/mnt/<drive>` dans WSL | Un seul `rm` ou `rmdir` dont les arguments sont des chemins relatifs sans `..`, `~`, jokers ni guillemets | Toute autre suppression littérale, notamment `git clean` et les chemins du système de fichiers propre à WSL | Répertoire de travail dans le système de fichiers propre à WSL |
| Codex déclaré Windows PowerShell | Un seul `Remove-Item` ou alias avec un chemin littéral ( `-Recurse` et `-Force` autorisés) | Toute autre suppression littérale : plusieurs instructions, pipelines, paramètres comme `-ErrorAction`, chemins de fournisseur, `~`, jokers ou backticks | Texte entre guillemets courbes |
| Codex sous Windows avec shell inconnu | — | Suppression dans un fragment raw de backtick ou `$(…)` | Autres commandes |
| pwsh sous Linux, `git rm` | — | — | Tout passe |

Pour obtenir un équivalent, la suppression doit être le **dernier segment** de la commande ; les segments précédents ne peuvent être reliés que par `&&`, `;` ou des retours à la ligne. Aucun équivalent n’est fourni pour les noms finissant par un point ou une espace, que Windows normaliserait. La suppression littérale reste bloquée.

## Utilisez-le vous-même

`obit rm`, `obit rmdir` et `obit git-clean` acceptent les options non interactives habituelles de `rm`, `rmdir` et `git clean`. Les options non prises en charge comme `-i` ou `rmdir -p` provoquent une erreur. `obit remove-item` est une version simple de `Remove-Item` qui accepte un seul chemin littéral ; pour les jokers, utilisez `obit rm *.log` dans Bash.

Désactivez temporairement avec `OBIT_GUARD_RM=0`. `obit stats` indique le nombre de suppressions arrêtées et la quantité enterrée.
