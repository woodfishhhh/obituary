# Conectar agentes

## Claude Code

```powershell
obit init -g                         # all projects (~/.claude); obit init alone installs into this project (.claude/)
obit init -g --show --agent claude   # what is installed
obit init -g --uninstall             # remove only what Obituary added
```

`obit init` registra `obit hook claude` em `settings.json` para SessionStart, antes de cada chamada de ferramenta, após as gravações, em Stop e em SessionEnd. Ele cria `OBIT.md` para o agente e acrescenta uma linha de referência a `CLAUDE.md`: `@OBIT.md` em `~/.claude/CLAUDE.md` no escopo do usuário (`-g`), ou `@.claude/OBIT.md` no `CLAUDE.md` do projeto para uma instalação local.

- As configurações e a ordem das chaves são preservadas. Executar o comando de novo não duplica entradas. Se você moveu o `obit.exe`, uma nova execução atualiza o caminho.
- Cada edição é precedida de um backup: `.bak` na primeira vez, depois `.bak.1`, `.bak.2` e assim por diante. Os backups anteriores nunca são sobrescritos.
- Se um hook falhar, o agente continua sem interrupção; o erro fica em `hook.log`. Use `OBIT_DISABLED=1` para desativar temporariamente os hooks.

## Codex

```powershell
obit init -g --agent codex   # ~/.codex/hooks.json plus a block in AGENTS.md; obit init --agent codex installs into this project
```

- **O Codex só executa hooks que você marcou como confiáveis.** Após a instalação, abra o Codex, execute `/hooks` e marque os hooks do Obituary como confiáveis. Para usar hooks de projeto, o próprio projeto também precisa ser confiável.
- Windows e WSL usam o mesmo `hooks.json`. Os caminhos `/mnt/<drive>/…` do WSL são convertidos em `<drive>:\…`. Por enquanto, os caminhos no sistema de arquivos próprio do WSL (`/home/…`) são ignorados.
- O Codex edita arquivos com `apply_patch`. O Obituary lê o patch para identificar arquivos adicionados, alterados ou movidos; os excluídos não contam.

## Outros agentes

```powershell
obit init -g --show                         # every supported agent, how far each integration goes, what is installed
obit init -g --agent gemini,cursor          # install one or several; without -g, into this project only
obit init -g --agent gemini --mcp --skill   # also register the MCP server and install the Skill (where supported)
obit init -g --agent gemini --uninstall     # remove only what Obituary added
```

Sem `--agent`, apenas a integração com o Claude Code é instalada. O Obituary não se instala em outros agentes por conta própria.

| Método | Agentes |
|---|---|
| Hooks nativos | GitHub Copilot (VS Code e Copilot CLI), Cursor, Gemini CLI, Factory Droid, Trae / Trae CN, Kimi, Mistral Vibe, Google Antigravity; Tencent CodeBuddy (CLI e IDE), Qoder / Qoder CN / Lingma, Zhipu ZCode |
| Plugin (o Obituary instala um pequeno plugin que chama `obit hook`) | OpenCode, OpenClaw, Pi, Oh My Pi, Hermes, Cline, Kilo Code, MiniMax Code, DeepSeek Harness |
| Arquivo de regras e MCP apenas | Roo Code, Windsurf, WorkBuddy, QoderWork |

::: tip O que cada agente suporta
Os agentes diferem no suporte ao registro após a gravação, ao aviso de início da sessão, ao guardião de exclusão, ao MCP e à Skill. A [tabela de compatibilidade](https://github.com/woodfishhhh/obituary/blob/main/docs/agents.md) do repositório indica o que tem suporte total, limitado ou nenhum e explica os motivos.
:::

Quando a documentação pública não basta para alterar a configuração com segurança, `obit init` mostra todas as etapas manuais e informa que a instalação está incompleta e precisa de intervenção. Não indica como instalado algo que não instalou.

## Servidor MCP e Agent Skill

```powershell
obit init -g --mcp --skill                 # Claude Code; add --agent codex for Codex. Neither is installed by default
```

- **`obit mcp`** é um servidor MCP que usa a entrada e a saída padrão (stdio). O agente pode chamar `obit_mark`, `obit_mktemp`, `obit_bury`, `obit_resurrect`, `obit_extend`, `obit_spare` e as ferramentas de consulta `obit_list`, `obit_graveyard`, `obit_proposals` e `obit_stats`.
- **As decisões são suas.** Não há ferramentas MCP para aprovar ou rejeitar propostas, confiar em `.obitrules`, excluir permanentemente itens do cemitério ou alterar configurações. O agente só pode encaminhar a proposta para você.
- A **Agent Skill** é um arquivo `SKILL.md` que orienta o agente sobre quando dar uma vida útil aos arquivos que produz, como excluí-los de forma reversível e quais decisões deixar para você. Se você editar a Skill, o Obituary não voltará a sobrescrevê-la nem a removê-la.
- Reinicie o agente após a instalação. `obit doctor` verifica se os dois componentes estão instalados e apontam para o `obit.exe` atual.
