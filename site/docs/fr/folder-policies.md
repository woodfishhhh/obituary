# Règles de dossier

Une règle de dossier attribue une durée de vie à chaque élément qui y arrive. Le dossier Téléchargements en est un exemple courant :

```powershell
obit watch C:\Users\you\Downloads 2w        # files that land in Downloads are buried two weeks later
obit watch --list                           # folder policies, their exemptions and quiet periods
obit watch --remove C:\Users\you\Downloads  # remove a policy and the lifespans it registered
```

## Conditions d’application

- **La règle doit être approuvée.** `obit watch` crée une proposition. La règle prend effet une fois que vous l’avez approuvée avec `obit approve` ou sur la page « Pour vous » de la petite fenêtre.
- **Le fichier doit être resté intact dix minutes.** Il n’est enterré que si sa durée de vie a expiré et s’il n’a pas été modifié au cours des dix dernières minutes.
- **Chaque sous-dossier fait l’objet d’une proposition distincte.** À son expiration, une proposition est créée. Il n’est enterré qu’après votre accord.
- Retirer une règle de dossier annule les durées de vie qu’elle a enregistrées.

## Dates dans l’Explorateur de fichiers {#dates-in-explorer}

Dans Téléchargements, sur le Bureau et dans les dossiers soumis à une règle, l’Explorateur affiche les éléments suivis sous la forme « name (11-13 burial) ». Le nom réel du fichier ne change pas. Le libellé suit la langue d’Obituary : si l’interface est en chinois simplifié, il prend la forme « name（11-13 下葬） » ; sinon, il reste en anglais.

- Les dates proviennent du fichier caché `desktop.ini` du dossier.
- Dans la section `[explorer]` de `config.toml`, utilisez `enabled = false` pour masquer les dates ou `everywhere = true` pour les afficher partout.
- L’Explorateur met les vues des dossiers en cache. Après une prolongation, l’ancienne date peut rester affichée dans la même fenêtre pendant une minute au maximum.
