# Conteneurs Docker

Les conteneurs démarrés dans une session d’agent peuvent également avoir une durée de vie.

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## Uniquement les conteneurs

Obituary ne gère que les conteneurs. Les volumes contiennent des données et leur suppression est irréversible. Obituary ne touche donc jamais aux volumes, aux images ni aux réseaux.

## À l’expiration, le conteneur s’arrête

- Après `docker stop`, le conteneur et sa couche accessible en écriture sont conservés. `obit resurrect <id>` lance `docker start`, puis met fin au suivi de sa durée de vie.
- Le conteneur n’est supprimé avec `docker rm` qu’après le délai de conservation (`[graveyard] retention_days`, 7 jours par défaut). Les options `-v` et `-f` ne sont jamais utilisées.
- S’il est en cours d’exécution à ce moment-là, que vous l’ayez redémarré vous-même ou non, il est considéré comme ressuscité et conservé.
- Pour les conteneurs, `obit purge` et `--all` respectent également le délai de conservation. Le mode corbeille ne s’applique pas : les conteneurs sont d’abord arrêtés, puis supprimés après ce délai.

## Identification par l’ID complet

- Le nom ne sert qu’à l’affichage. Avant tout arrêt, démarrage ou retrait, Obituary vérifie à nouveau l’ID complet et l’heure de création. Si l’un des deux ne correspond pas à l’enregistrement, il ne fait rien.
- Obituary enregistre le contexte Docker et le démon dont dépend le conteneur. Il n’agit que sur ce démon : changer de `docker context` ne redirige pas ses opérations.
- Si vous supprimez un conteneur en dehors d’Obituary, la suppression externe est consignée et l’entrée de suivi retirée.
- Docker supprime à l’arrêt les conteneurs lancés avec `--rm` (AutoRemove). Obituary refuse donc de les enregistrer.

## Enregistrement automatique des conteneurs d’une session

Par défaut, l’enregistrement automatique est désactivé : Obituary ne recherche pas de conteneurs. Vous pouvez en enregistrer un avec `obit docker add`. Si vous activez l’option, Obituary recherche en arrière-plan, à la fin de chaque session, les conteneurs créés pendant celle-ci. Seuls sont retenus ceux dont le répertoire de travail Compose ou un montage de type bind se trouve dans le projet. Les autres ont pu être lancés ailleurs et ne sont pas touchés. Les conteneurs retenus reçoivent une durée de vie de trois jours ; vous en êtes informé après coup. Si vous annulez avec `obit spare <id>`, la session n’enregistre plus ce conteneur. Si la session s’est déroulée hors d’un projet Git (dans votre répertoire personnel, par exemple), ou si plusieurs sessions revendiquent le même conteneur, son origine est incertaine : une proposition attend votre accord. Avec `[proposals] mode = "ask"`, il y a également une proposition et le suivi ne commence qu’après `obit approve`.

Un conteneur fait l’objet d’une proposition au maximum. Les hooks n’appellent jamais Docker. Si Docker n’est pas installé ou ne tourne pas, cette étape est ignorée sans notification ; `obit doctor` en indique la raison. Tant que Docker est arrêté, les propositions concernant les conteneurs ne peuvent pas être approuvées. `obit proposals` et la page des décisions de la fenêtre le signalent. Vous pourrez les approuver une fois Docker démarré.
