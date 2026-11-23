# Quels fichiers enregistrer

## L’agent et vous décidez ; Obituary enterre à temps

L’agent et vous décidez de ce qui entre dans Obituary (par défaut, `[proposals] mode = "off"`) :

- **L’agent enregistre ses propres productions.** Il attribue une durée avec `obit mark <path> 2d` à ses scripts de débogage, journaux et captures, crée un espace temporaire avec `obit mktemp 1d` et enterre avec `obit bury` ce qu’il ne sait pas devoir conserver. `obit init` lui explique cette règle.
- **Vous ajoutez des éléments depuis le menu contextuel.** Sous Windows, faites un clic droit sur des fichiers ou dossiers dans l’Explorateur, choisissez « Add to Obituary », puis 1, 7, 30 jours ou « Bury now (recoverable) ». Voir [Fenêtre, icône de notification et menu contextuel](./desktop).
- **Les règles de dossier** attribuent une durée de vie à des dossiers comme Téléchargements. Voir [Règles de dossier](./folder-policies).

Tout élément enregistré est enterré à l’échéance et reste ressuscitable à l’identique pendant sept jours. Rien d’autre n’entre automatiquement.

- **Nettoyage en fin de session.** Quand un agent Claude Code ou Codex va s’arrêter, Obituary liste les fichiers créés pendant la session qui ne sont pas classés. Il lui demande d’attribuer une durée ou d’enterrer scripts, journaux et captures, en conservant les livrables. Une seule notification par session, silencieuse s’il n’y a rien ; les autres agents la reçoivent au début de la session suivante. `[hooks] session_cleanup = false` la désactive.
- **Lors d’une mise à niveau depuis une ancienne version**, ce réglage est désactivé une fois. Les dépendances, sorties de compilation et caches enregistrés automatiquement avant restent sur place, sans durée de vie.

## Activez l'enregistrement automatique quand vous le souhaitez

Activez-le dans Settings ou For you, ou définissez `[proposals] mode` dans `config.toml`.

- `"tiered"` enregistre directement les fichiers créés par l’agent pendant la session et les [conteneurs Docker](./docker) démarrés pour le projet. Un fichier doit avoir été observé par le hook, être absent de l’instantané initial et avoir une date de création postérieure au début de la session. L’agent est informé et l’élément apparaît dans `obit proposals` et sous « Handled automatically » dans For you. Les fichiers déjà présents ou non observés par le hook deviennent seulement des propositions. `obit approve <id>` enregistre les éléments proposés ; les correspondances de règles du projet sont ensuite enregistrées directement. `obit reject <id>` refuse l’enregistrement automatique dans ce projet.
- `"auto"` enregistre directement tout ce que trouve l’enregistrement automatique et vous en informe ensuite.
- `"ask"` transforme tout en proposition en attente d’approbation.

```powershell
obit proposals           # proposals waiting for you, and what was registered without asking
obit approve 3           # approve: register what it lists
obit reject 3            # reject
obit spare 12            # undo an automatic registration: it stays where it is, without a lifespan
obit extend 12 1w        # keep it longer
```

- **Dépendances, sorties de compilation et caches** (`node_modules`, `dist`, `.turbo`, `__pycache__`, etc.) ne sont jamais enregistrés ni proposés automatiquement. Pour les nettoyer, utilisez vous-même `obit mark`.
- **Les éléments volumineux** que l’agent n’a pas créés sont aussi proposés s’ils dépassent 1 Go ou 200 fichiers.
- **Différence de session.** Un instantané est pris au début, conservé dans le répertoire de données sans rien ajouter au dépôt, puis comparé à la fin. Les correspondances de règles suivent le mode ; les nouveaux fichiers sans règle ne sont ni enregistrés ni proposés.
- **Après un changement de mode**, le processus d’arrière-plan reclasse les propositions en attente.
- **Git est consulté une nouvelle fois avant l’enterrement.** Un élément enregistré par règle ou proposition n’est pas enterré s’il figure dans un commit à l’échéance.
- **Les propositions expirent automatiquement.** Les éléments validés ou supprimés sont retirés ; une proposition vide est supprimée, inscrite dans le journal d’audit et ne compte pas comme un refus.

Les volumes ne sont jamais enregistrés automatiquement. Obituary ne traite que ceux que vous enregistrez ou enterrez explicitement et ne supprime ni les images ni les réseaux.

## Règles

```powershell
obit rules                       # the rules in effect here (built-in plus a trusted .obitrules)
obit rules check debug.log out/  # preview what would be registered and by which rule (changes nothing)
obit rules apply debug.log       # register by rule
```

`obit rules apply` enregistre selon les règles à tout moment ; avec l’enregistrement automatique activé, les hooks les appliquent aussi. Les règles intégrées couvrent les fichiers de travail fréquents comme `.playwright-mcp/`, `test-results/` et `*.log`. Les sauvegardes `*.bak`, `*.bak.*`, `*.orig`, `*.rej`, `*.old`, `*.backup` et `*~` sont conservées sept jours, tandis que `LOG.old` de LevelDB est laissé intact. Le fichier `.obitrules` à la racine du projet suit la syntaxe `.gitignore` et indique une durée après chaque motif.

```
output/**        7d      # every item under output gets its own lifespan
!output/keep/**          # never touch (negation wins, even over registering a parent folder)
test-results/    off     # turn off a built-in rule
```

Contrairement à `.gitignore`, les exclusions sont prioritaires sur la dernière correspondance. Windows ignore la casse. Un motif invalide, comme `[` non fermé, signale le fichier et la ligne et suspend l’enregistrement automatique du projet.

Les unités sont `s`, `m`, `h`, `d` et `w` ; elles se combinent, par exemple `1d12h`.

## Règles strictes pour l'enregistrement automatique

- Seuls les éléments non suivis dans un arbre de travail Git sont enregistrés.
- Un dossier qui contient au moins un fichier suivi n’est jamais touché.
- Si Git ne répond pas, l’élément est ignoré.
- Une durée déjà définie n’est jamais raccourcie.
- Un projet que vous avez refusé n’est jamais enregistré directement. Les règles de dossier et la confiance dans `.obitrules` attendent toujours votre décision.

## Les règles de projet doivent être approuvées en premier

Le `.obitrules` du dépôt reste inactif tant que vous ne l’avez pas examiné et exécuté `obit trust`. Ainsi, un dépôt tiers ne peut pas utiliser ses règles pour prendre vos fichiers privés. Le SHA-256 du fichier est mémorisé ; toute modification exige une nouvelle vérification. Les sessions non interactives nécessitent `--yes` et la CI peut définir `OBIT_TRUST_PROJECT_RULES=1`.

## Un dossier temporaire qui s'enfonce

```powershell
$dir = obit mktemp 1d            # .obit/tmp/ in a project (not in Git), otherwise the system temp folder
```

Dans bash: `dir=$(obit mktemp 1d)`.
