# Referência de configuração

O Obituary guarda suas configurações em `config.toml`, no diretório de configuração. O arquivo é opcional: sem ele, ou com apenas algumas linhas, todo o resto usa o valor padrão. `obit config` mostra onde o arquivo está, se ele existe e o valor que cada configuração tem de fato neste momento. `obit config --create` escreve um arquivo comentado com todos os valores padrão, que é o jeito mais simples de começar a editar.

O diretório de configuração é `%APPDATA%\obituary` no Windows, `~/Library/Application Support/obituary` no macOS e `~/.config/obituary` no Linux (ou `obituary` dentro de `XDG_CONFIG_HOME`, quando ela estiver definida). Para colocá-lo em outro lugar, defina a variável de ambiente `OBIT_CONFIG_DIR`.

| Chave | Valor padrão | Valores aceitos |
| --- | --- | --- |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` |
| `graveyard.retention_days` | `7` | um inteiro de 1 a 3650 |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` |
| `hooks.session_cleanup` | `true` | `true`, `false` |
| `hooks.session_summary` | `false` | `true`, `false` |
| `watch.stable_window` | `10m` | uma duração positiva, como `30s`, `10m`, `1h30m` |
| `explorer.enabled` | `true` | `true`, `false` |
| `explorer.everywhere` | `false` | `true`, `false` |
| `explorer.in_git` | `false` | `true`, `false` |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` |
| `ui.tray` | `true` | `true`, `false` |
| `ui.taskbar` | `true` | `true`, `false` |
| `update.check` | `true` | `true`, `false` |
| `telemetry.enabled` | `true` | `true`, `false` |
| `agent_shells` | não definida | uma tabela: nome do agente = `bash` ou `powershell` |
| `codex_windows_shell` | não definida | `bash`, `powershell` |

## Para onde vai o que expira

Por padrão, o que esgota a vida útil vai para o cemitério do Obituary. Durante `graveyard.retention_days` dias (7, se você não mudar) é possível devolvê-lo ao lugar com `obit resurrect`; só depois desse prazo ele é excluído de vez. O número pode ser qualquer valor entre 1 e 3650.

Se você definir `graveyard.mode` como `recycle`, o que expira vai para a Lixeira do sistema, e quem cuida dela é o Windows. Isso existe só no Windows; macOS e Linux têm apenas o cemitério. O que está na Lixeira é restaurado a partir da Lixeira, e `obit resurrect` não o encontrará.

## Registro automático e sessões de agentes

`proposals.mode` decide se o Obituary dá vida útil aos arquivos por conta própria. O padrão, `off`, não faz nada automaticamente: só entra o que um agente ou você registram por comandos, pelo menu de contexto ou por políticas de pastas. Com `tiered`, os arquivos que o agente criou nesta sessão e os contêineres que a sessão iniciou entram na hora e você é avisado depois; `obit spare` desfaz um deles. Todo o resto espera sua aprovação em `obit proposals`. `auto` registra tudo o que o registro automático encontra e avisa depois. `ask` pergunta primeiro sobre cada item. Em qualquer modo, arquivos que o Git já acompanha, dependências, saídas de compilação e caches (`node_modules`, `dist`, `__pycache__` e semelhantes) nunca são registrados automaticamente. As regras estão em [Quais arquivos são registrados](./auto-register).

`hooks.session_cleanup` vem ativado. Perto do fim de cada sessão, o agente fica sabendo quais arquivos criou que não têm vida útil e que o Git não acompanha, e é convidado a dar vida útil aos arquivos temporários (`obit mark`) ou a enterrá-los; os entregáveis ficam. `hooks.session_summary` vem desativado; se você o ativar, no início da sessão o agente também fica sabendo quais itens do projeto expiram em 24 horas e o que foi enterrado recentemente. Ele fica desativado para economizar tokens, porque o agente é avisado de qualquer forma quando toca um caminho enterrado.

## Políticas de pastas e datas no Explorador de Arquivos

`watch.stable_window` é a paciência de uma política de pasta: um arquivo novo precisa ficar sem mudanças por esse tempo antes de ser movido, para que nada seja levado enquanto ainda está sendo baixado. Ela usa o formato de duração do Go, um número seguido de `s`, `m` ou `h`, que podem ser encadeados como em `1h30m`, e deve ser maior que zero.

As configurações `explorer.*` só importam no Windows. O Obituary faz o Explorador de Arquivos mostrar a data de sepultamento ao lado do nome do arquivo, sem alterar o nome real. Por padrão isso acontece em Downloads, na Área de Trabalho (incluindo subpastas) e nos lugares que uma política de pasta gerencia. Com `explorer.everywhere` ativado, outros locais acompanhados também mostram a data; áreas de trabalho do Git ficam de fora, a menos que você defina `explorer.in_git` como `true`; e `explorer.enabled = false` desliga tudo. Veja [Políticas de pastas](./folder-policies).

## Interface

`ui.language` define o idioma da linha de comando, da janela, do ícone da bandeja e do menu de contexto. Por enquanto só existem `en` e `zh-CN`. `auto` consulta a variável de ambiente `OBIT_LANG`, depois `LC_ALL`, `LC_MESSAGES` e `LANG` e, por fim, o idioma da interface do sistema. `ui.tray` mostra ou oculta o ícone da bandeja (o ícone da barra de menus no macOS); oculto, a limpeza em segundo plano e as verificações de atualização continuam. `ui.taskbar` vale só para o Windows e decide se a janela ocupa um botão na barra de tarefas enquanto estiver aberta.

## Verificação de atualizações e estatísticas de uso

Com `update.check` ativado, o Obituary lê as informações públicas de versões no GitHub no máximo uma vez por dia e não envia dados de uso. Com ele desativado, você ainda pode executar `obit update --check` quando quiser. `telemetry.enabled` controla as estatísticas de uso anônimas. É um interruptor separado do das atualizações: `obit config telemetry off` desativa só as estatísticas e apaga deste computador o ID de instalação e qualquer relatório pendente. Os detalhes estão na [política de privacidade](./privacy).

## Qual shell um agente usa no Windows

O guardião de exclusão precisa ler o comando de exclusão que um agente está prestes a executar para poder sugerir o comando `obit` equivalente. No Windows, um agente pode usar bash ou PowerShell, e o Obituary não consegue saber qual, então você declara: este agente roda neste computador Windows, neste shell. Sem declaração, comandos de significado ambíguo passam sem ser bloqueados.

```toml
[agent_shells]
codex = "powershell"
```

As chaves de `agent_shells` são nomes de agentes. Hoje quem a lê são `codex`, `codebuddy`, `codebuddy-ide` e `minimax-code`. `codex_windows_shell` é a forma antiga, só para o Codex; se ela e `agent_shells.codex` estiverem definidas com valores diferentes, a configuração é rejeitada.

## Como aparece um erro

O Obituary não ignora em silêncio uma chave que não conhece. Se ignorasse, `retention = 30` deixaria em vigor a retenção de 7 dias enquanto você acredita que ela passou a 30. Por isso `obit config` informa o caminho do arquivo, o número da linha e o nome completo do que não reconhece, e sugere a chave certa quando a grafia é parecida:

```text
obit: config file has errors: …\config.toml:2: unknown key "graveyard.retention" (did you mean "graveyard.retention_days"?)
```

Um valor fora do intervalo é relatado do mesmo jeito, com a configuração e o que ela aceita, por exemplo `graveyard.retention_days must be between 1 and 3650`. Um erro de sintaxe TOML também faz `obit config` falhar. Corrija o arquivo e execute de novo; quando ele deixar de reclamar, está resolvido.

## Comandos comuns

```powershell
obit config                  # local do arquivo, diretório de dados e os valores em vigor
obit config --create         # escreve o arquivo padrão comentado
obit config --language en    # salva o idioma da interface: auto, en ou zh-CN
obit config telemetry off    # desativa as estatísticas de uso (on volta a ativá-las)
```

`--create` nunca sobrescreve um arquivo existente; se o arquivo já estiver lá, ele relata um erro e para. `--language` cria o arquivo com os valores padrão quando ele ainda não existe.
