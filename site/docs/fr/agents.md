# Connecter les agents

## Claude Code

```powershell
obit init -g                         # all projects (~/.claude); obit init alone installs into this project (.claude/)
obit init -g --show --agent claude   # what is installed
obit init -g --uninstall             # remove only what Obituary added
```

`obit init` inscrit `obit hook claude` dans `settings.json` pour SessionStart, avant chaque appel d’outil, après les écritures, à Stop et à SessionEnd. Il crée `OBIT.md` pour l’agent et ajoute une ligne de référence à `CLAUDE.md` : `@OBIT.md` dans `~/.claude/CLAUDE.md` pour l’installation utilisateur (`-g`), ou `@.claude/OBIT.md` dans le `CLAUDE.md` du projet pour une installation locale.

- Vos réglages et l’ordre des clés sont préservés. Relancer la commande ne crée pas de doublons ; si `obit.exe` a été déplacé, cela met son chemin à jour.
- Une sauvegarde précède chaque modification : `.bak` la première fois, puis `.bak.1`, `.bak.2`, etc. Les anciennes sauvegardes ne sont jamais écrasées.
- En cas d’erreur dans un hook, l’agent continue sans interruption. L’erreur est consignée dans `hook.log`. `OBIT_DISABLED=1` désactive temporairement les hooks.

## Codex

```powershell
obit init -g --agent codex   # ~/.codex/hooks.json plus a block in AGENTS.md; obit init --agent codex installs into this project
```

- **Codex n’exécute que les hooks auxquels vous avez accordé votre confiance.** Après l’installation, ouvrez Codex, lancez `/hooks` et approuvez les hooks d’Obituary. Pour les hooks de projet, le projet lui-même doit aussi être de confiance.
- Un seul fichier `hooks.json` sert à Windows et à WSL. Les chemins WSL de la forme `/mnt/<drive>/…` sont convertis en `<drive>:\…`. Pour le moment, les chemins dans le système de fichiers propre à WSL (`/home/…`) sont ignorés.
- Codex modifie les fichiers avec `apply_patch`. Obituary lit le patch pour repérer les ajouts, les modifications et les déplacements ; les fichiers supprimés ne comptent pas.

## Autres agents

```powershell
obit init -g --show                         # every supported agent, how far each integration goes, what is installed
obit init -g --agent gemini,cursor          # install one or several; without -g, into this project only
obit init -g --agent gemini --mcp --skill   # also register the MCP server and install the Skill (where supported)
obit init -g --agent gemini --uninstall     # remove only what Obituary added
```

Sans `--agent`, seule l’intégration à Claude Code est installée. Obituary ne s’installe jamais de sa propre initiative dans un autre agent.

| Méthode | Agents |
|---|---|
| Hooks natifs | GitHub Copilot (VS Code et Copilot CLI), Cursor, Gemini CLI, Factory Droid, Trae / Trae CN, Kimi, Mistral Vibe, Google Antigravity ; Tencent CodeBuddy (CLI et IDE), Qoder / Qoder CN / Lingma, Zhipu ZCode |
| Plugin (Obituary installe un petit plugin qui appelle `obit hook`) | OpenCode, OpenClaw, Pi, Oh My Pi, Hermes, Cline, Kilo Code, MiniMax Code, DeepSeek Harness |
| Fichier de règles et MCP uniquement | Roo Code, Windsurf, WorkBuddy, QoderWork |

::: tip Fonctionnalités disponibles selon l’agent
Les agents ne prennent pas tous en charge l’enregistrement après écriture, l’avis de début de session, le garde de suppression, MCP et la Skill. Le [tableau de compatibilité](https://github.com/woodfishhhh/obituary/blob/main/docs/agents.md) du dépôt indique pour chacun « pris en charge », « limité » ou « non pris en charge », avec les raisons.
:::

Si la documentation publique ne permet pas de modifier la configuration en toute sécurité, `obit init` affiche la procédure manuelle complète et signale que l’installation reste à terminer à la main. Rien n’est déclaré installé si ce n’est pas le cas.

## Serveur MCP et Agent Skill

```powershell
obit init -g --mcp --skill                 # Claude Code; add --agent codex for Codex. Neither is installed by default
```

- **`obit mcp`** est un serveur MCP sur l’entrée et la sortie standard (stdio). L’agent peut appeler `obit_mark`, `obit_mktemp`, `obit_bury`, `obit_resurrect`, `obit_extend`, `obit_spare` et les outils de consultation `obit_list`, `obit_graveyard`, `obit_proposals` et `obit_stats`.
- **Ces décisions vous appartiennent.** Aucun outil MCP ne permet d’accepter ou de refuser une proposition, de faire confiance à `.obitrules`, de supprimer définitivement des éléments du cimetière ou de modifier la configuration. L’agent peut seulement vous transmettre la proposition.
- L’**Agent Skill** est un fichier `SKILL.md` qui explique quand attribuer une durée de vie aux fichiers produits, comment les supprimer de manière réversible et quelles décisions vous laisser prendre. Si vous le modifiez, Obituary ne l’écrase plus et ne le supprime plus.
- Redémarrez l’agent après l’installation. `obit doctor` vérifie que les deux composants sont installés et pointent vers le fichier `obit.exe` actuel.
