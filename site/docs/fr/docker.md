# Conteneurs et volumes Docker

Les conteneurs démarrés dans une session d’agent peuvent également avoir une durée de vie.

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## Attribuer une durée à un volume ou l’enterrer immédiatement

Les volumes ne sont jamais enregistrés automatiquement. Obituary ne traite que ceux que vous enregistrez ou enterrez explicitement et ne supprime ni les images ni les réseaux.

Avec `obit mark --docker-volume cache 3d` (ou `add`), le volume `cache` sera enterré dans trois jours. Pour l’enterrer maintenant, utilisez `obit bury --docker-volume -- cache`.

Docker doit être en mode conteneurs Linux. Seuls les volumes locaux utilisant le pilote `local` sans options supplémentaires sont pris en charge. Au premier usage, Docker télécharge une image Debian de version fixe si elle n’est pas déjà présente.

Prévoyez assez de place au cimetière pour une archive complète. Le volume n’est supprimé qu’après archivage et vérification. Si un conteneur, même arrêté, référence le volume, ou si son chemin recoupe un montage de type bind, il reste intact.

`obit resurrect <id>` recrée le volume sous son nom d’origine avec le label permanent `io.obituary.restore`. Si ce nom existe déjà, le volume n’est pas écrasé, même vide.

Les événements Docker sont vérifiés avant et après la suppression. Si celle-ci ne peut pas être confirmée, gardez l’archive vérifiée et consultez `obit doctor` ; ne considérez pas le volume comme enterré. Chaque avertissement `docker-volume-recovery` signale une entrée à vérifier. Suivez les indications pour examiner le volume sur le démon Docker d’origine. Ni `obit resurrect` ni `obit purge` ne prennent en charge une archive non inscrite au registre.

La protection reconnaît `docker volume rm`, `docker volume remove`, `docker volume prune`, `docker compose down -v` (ou `--volumes`) et `docker system prune --volumes`. Elle ne propose `obit bury --docker-volume` que pour un simple `docker volume rm` avec des noms explicites.

## À l’expiration, le conteneur s’arrête

- Après `docker stop`, le conteneur et sa couche accessible en écriture sont conservés. `obit resurrect <id>` lance `docker start`, puis met fin au suivi de sa durée de vie.
- Le conteneur n’est supprimé avec `docker rm` qu’après le délai de conservation (`[graveyard] retention_days`, 7 jours par défaut). Les options `-v` et `-f` ne sont jamais utilisées.
- Si vous démarrez le conteneur pendant la période de conservation, Obituary le conserve et cesse de le suivre, même si vous l’arrêtez de nouveau. Les anciennes versions n’enregistraient pas l’heure de démarrage : ces conteneurs ne sont pas supprimés et restent récupérables avec `obit resurrect <id>`.
- Pour les conteneurs, `obit purge` et `--all` respectent également le délai de conservation. Le mode corbeille ne s’applique pas : les conteneurs sont d’abord arrêtés, puis supprimés après ce délai.

## Identification par l’ID complet

- Le nom ne sert qu’à l’affichage. Avant tout arrêt, démarrage ou retrait, Obituary vérifie à nouveau l’ID complet et l’heure de création. Si l’un des deux ne correspond pas à l’enregistrement, il ne fait rien.
- Obituary enregistre le contexte Docker et le démon dont dépend le conteneur. Il n’agit que sur ce démon : changer de `docker context` ne redirige pas ses opérations.
- Si vous supprimez un conteneur en dehors d’Obituary, la suppression externe est consignée et l’entrée de suivi retirée.
- Docker supprime à l’arrêt les conteneurs lancés avec `--rm` (AutoRemove). Obituary refuse donc de les enregistrer.

## Enregistrement automatique des conteneurs d’une session

Par défaut, l’enregistrement automatique est désactivé : Obituary ne recherche pas de conteneurs. Vous pouvez en enregistrer un avec `obit docker add`. Si vous activez l’option, Obituary recherche en arrière-plan, à la fin de chaque session, les conteneurs créés pendant celle-ci. Seuls sont retenus ceux dont le répertoire de travail Compose ou un montage de type bind se trouve dans le projet. Les autres ont pu être lancés ailleurs et ne sont pas touchés. Les conteneurs retenus reçoivent une durée de vie de trois jours ; vous en êtes informé après coup. Si vous annulez avec `obit spare <id>`, la session n’enregistre plus ce conteneur. Si la session s’est déroulée hors d’un projet Git (dans votre répertoire personnel, par exemple), ou si plusieurs sessions revendiquent le même conteneur, son origine est incertaine : une proposition attend votre accord. Avec `[proposals] mode = "ask"`, il y a également une proposition et le suivi ne commence qu’après `obit approve`.

Un conteneur fait l’objet d’une proposition au maximum. Les hooks n’appellent jamais Docker. Si Docker n’est pas installé ou ne tourne pas, cette étape est ignorée sans notification ; `obit doctor` en indique la raison. Tant que Docker est arrêté, les propositions concernant les conteneurs ne peuvent pas être approuvées. `obit proposals` et la page des décisions de la fenêtre le signalent. Vous pourrez les approuver une fois Docker démarré.
