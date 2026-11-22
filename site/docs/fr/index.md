# Démarrage rapide

Obituary donne une durée de vie aux fichiers et aux dossiers. Lorsqu’elle arrive à son terme, ils sont enterrés dans un cimetière ; vous pouvez les ressusciter à tout moment pendant la période de conservation (7 jours par défaut), et ils ne sont réellement supprimés qu’après cette période. Obituary est conçu pour les agents d’IA : les scripts de débogage, journaux, captures d’écran et sorties de tests qu’un agent produit ont une durée de vie dès leur création. Personne n’a besoin de penser au nettoyage, et le `rm -rf` d’un agent ne détruit plus rien définitivement.

## 1. Installer

Windows 10 / 11, macOS et Linux (x64 et ARM64) :

| | Windows | macOS | Linux |
|---|---|---|---|
| Installer avec | Scoop, le script `install.ps1` (winget en cours d’examen) | Homebrew, le script `install.sh` | le script `install.sh` |
| Vérification des échéances | processus résident dans la zone de notification, Planificateur de tâches en secours | agent launchd de l’utilisateur | minuteur `systemd --user` ; crontab utilisateur sans systemd |
| CLI, cimetière et résurrection, enregistrement automatique (facultatif), garde de suppression, intégration aux agents (hooks, Skill, MCP), conteneurs Docker, politiques de dossiers | oui | oui | oui |
| Zone de notification, petite fenêtre, menus contextuels, dates d’enterrement dans l’Explorateur, menu Démarrer et raccourcis du bureau | oui | utilisez plutôt la CLI : `obit list`, `obit graveyard`, `obit resurrect` | comme sur macOS |
| Mode Corbeille | oui | cimetière uniquement | cimetière uniquement |

**Le plus simple : laissez votre agent s’en charger.** Envoyez cette phrase à l’agent que vous utilisez (Claude Code, Codex, Cursor, etc.). Il installe Obituary, se connecte à lui-même et s’arrête lorsque la décision vous revient.

```text
Read https://obituary.woodfish.site/agent-install.md and follow it to install Obituary and connect it to yourself.
```

**Manuellement :** choisissez votre système d’exploitation ci-dessous, puis exécutez `obit doctor` pour vérifier l’installation.

### macOS / Linux

Sur macOS, Homebrew l’installe en une seule ligne :

```bash
brew install --cask woodfishhhh/tap/obituary
```

Vous pouvez aussi télécharger et lire le script d’installation, puis l’exécuter (macOS et Linux) :

```bash
curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh
bash install.sh
# Custom directory: --install-dir "$HOME/bin"; preview only: --dry-run
```

Le script installe la dernière version par défaut (`--version <version>` permet d’en choisir une autre). L’archive est téléchargée en quatre parties simultanément (certains itinéraires limitent le débit de chaque connexion) et revient à une seule connexion en cas d’échec. Il détecte le système d’exploitation et l’architecture, vérifie le SHA-256 depuis la même source de téléchargement, puis installe `obit` et `obitd` dans `~/.local/bin`. Suivez les instructions concernant le PATH, puis exécutez `obit --version`. Utilisez `--mirror cn` pour le miroir de Chine continentale ; son script se trouve à `https://cn.obituary.woodfish.site/install.sh`.

Lors de la première utilisation sur macOS, exécutez `obit --version` et `obitd --help`. Si Gatekeeper demande une autorisation, ouvrez Réglages Système → Confidentialité et sécurité, choisissez « Ouvrir quand même » pour chaque exécutable dont vous avez vérifié la provenance, puis réessayez.

Sous WSL, gardez les projets et la configuration de l’agent sur le système de fichiers Linux (par exemple sous `~/projects`). Les montages Windows tels que `/mnt/c` ne prennent pas toujours en charge les renommages sécurisés nécessaires à l’installation ; l’opération signale une erreur et conserve la configuration d’origine. Vous pouvez aussi utiliser directement la version Windows d’Obituary.

### Windows

::: code-group

```powershell [Scoop]
scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket
scoop install obituary
```

```powershell [winget]
# In review, available once approved
winget install woodfishhhh.Obituary
```

```powershell [Install script]
# Install script: download it, read it, then run it
# (installs to %LOCALAPPDATA%\Programs\obituary, adds it to your user PATH, creates a Start menu shortcut)
irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1
.\install.ps1          # installs the latest release; -WhatIf previews, -NoPath leaves PATH alone
```

:::

- Les trois méthodes téléchargent la même archive depuis la release GitHub et vérifient son SHA-256 avant la décompression ; toute incohérence arrête l’installation.
- Signature du code : [SignPath.io](https://about.signpath.io/) fournit gratuitement la signature du code, et le certificat est fourni par [SignPath Foundation](https://signpath.org/). Consultez la [politique de signature du code](code-signing) et la [politique de confidentialité](privacy).
- Obituary n’est pas encore signé. Si vous téléchargez l’archive dans un navigateur, Windows peut afficher « Windows a protégé votre ordinateur » ; choisissez « Plus d’informations → Exécuter quand même ». Scoop, winget et le script d’installation ne déclenchent généralement pas cette alerte.
- La petite fenêtre `obit-gui.exe` nécessite le runtime WebView2 (intégré à Windows 11 et présent sur la plupart des machines Windows 10) et fonctionne uniquement en x64 ; la version ARM64 inclut la ligne de commande et la zone de notification.
Vérifiez ensuite l’installation :

```powershell
obit doctor              # read-only check: PATH, obitd, runner, Claude Code / Codex integration, data directory, ledger, config (--json)
```

## 2. Connecter vos agents

```powershell
obit init -g                 # Claude Code, all projects; also the Skill (and desktop integration on Windows)
obit init -g --agent codex   # Codex
```

Redémarrez ensuite l’agent. Pour les autres agents, le serveur MCP et l’Agent Skill, consultez [Connecter les agents](./agents).

Sous macOS / Linux, Claude Code utilise `~/.claude`, Codex utilise `~/.codex` et les Skills partagées utilisent `~/.agents/skills`. Les remplacements `CLAUDE_CONFIG_DIR` et `CODEX_HOME` sont également pris en compte.

Avec la supervision du bureau activée, le processus résident traite les échéances à temps et une tâche indépendante vérifie les retards toutes les heures. Sans supervision de l’icône pour ce répertoire de données, la tâche s’exécute toutes les cinq minutes.

```bash
obit daemon enable --dry-run
obit daemon enable
obit doctor
```

macOS utilise le launchd de l’utilisateur connecté ; Linux utilise `systemd --user`. Sous Linux sans systemd, planifiez `obitd --once` toutes les 5 minutes dans le crontab de l’utilisateur et fournissez les chemins absolus vers `obitd`, le répertoire de données `--home` et le répertoire de configuration `--config-dir`. Les hooks de l’agent lancent aussi une vérification en arrière-plan lorsqu’une tâche arrive à échéance.

## 3. Commandes courantes

Remplacez `<id>` par le numéro d’un reçu de `obit add` ou `obit bury`, ou par le numéro affiché par `obit list` / `obit graveyard`.

```powershell
obit add debug.log 2h        # bury it in 2 hours (alias: mark)
obit list                    # what is left and for how long
obit extend <id> 1d          # one more day
obit bury old-build          # bury right away
obit graveyard               # what is in the graveyard and when it will be purged
obit resurrect <id>          # put it back, never overwriting
obit resurrect --last        # undo the last deletion
```

Pour tout le reste, consultez [Commandes](./commands).

## Ensuite

| Pour savoir | Lire |
|---|---|
| Comment fonctionnent l’enregistrement, l’enterrement, la résurrection et la suppression définitive, et où se trouve le cimetière | [Fonctionnement](./how-it-works) |
| Connecter Claude Code, Codex et d’autres agents | [Connecter les agents](./agents) |
| Ce qui est enregistré : enregistrements d’agents, menu contextuel, enregistrement automatique facultatif | [Ce qui est enregistré](./auto-register) |
| Transformer le `rm -rf` d’un agent en enterrement réversible | [Garde de suppression](./delete-guard) |
| Ce qui se passe lorsqu’un agent accède à un fichier enterré | [Notifications du cimetière](./grave-notices) |
| Durées de vie des dossiers comme Téléchargements et Bureau | [Politiques de dossiers](./folder-policies) |
| Utiliser Obituary sans la ligne de commande | [Fenêtre, zone de notification et menu contextuel](./desktop) |
| Conteneurs Docker lancés pendant une session | [Conteneurs Docker](./docker) |
| Toutes les commandes | [Commandes](./commands) |
| Supprimer Obituary | [Désinstaller](./uninstall) |
| Tout le reste | [FAQ](./faq) |
