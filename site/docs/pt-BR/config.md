# Referência de configuração

O Obituary lê `config.toml` no diretório de configuração. Todas as chaves são opcionais; as ausentes usam os padrões abaixo. `obit config` mostra o caminho e os valores efetivos.

## Chaves

| Chave | Padrão | Valores ou formato | Efeito |
| --- | --- | --- | --- |
| `codex_windows_shell` | não definido | `bash`, `powershell` | Declara o shell usado pelo Codex no Windows; comandos de exclusão ambíguos passam sem alteração quando ausente. |
| `agent_shells.<agent>` | não definido | `bash`, `powershell` | Declara o shell Windows de um hook agent. |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` | Move itens vencidos para o cemitério ou para a lixeira do sistema. |
| `graveyard.retention_days` | `7` | Inteiro de `1` a `3650` | Dias em que um item do cemitério pode ser restaurado. |
| `watch.stable_window` | `10m` | Duração Go, como `30s`, `10m`, `1h` | Aguarda a estabilidade antes de mover itens por uma política de pasta. Deve ser positiva. |
| `explorer.enabled` | `true` | `true`, `false` | Escreve rótulos de data no Explorer. |
| `explorer.everywhere` | `false` | `true`, `false` | Mostra datas em todas as pastas compatíveis. |
| `explorer.in_git` | `false` | `true`, `false` | Mostra datas dentro de worktrees Git. |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` | Idioma do CLI, da janela e da bandeja. |
| `ui.tray` | `true` | `true`, `false` | Ativa a entrada da bandeja ou da barra de menus. |
| `ui.taskbar` | `true` | `true`, `false` | Dá à janela do Windows seu próprio botão na barra de tarefas. |
| `hooks.session_summary` | `false` | `true`, `false` | Mostra um resumo ao iniciar uma sessão. |
| `hooks.session_cleanup` | `true` | `true`, `false` | Organiza arquivos criados por esta sessão que continuem sem classificação. |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` | Define quais registros automáticos aguardam aprovação. |
| `update.check` | `true` | `true`, `false` | Verifica automaticamente versões estáveis. |
| `telemetry.enabled` | `true` | `true`, `false` | Permite estatísticas de uso; é independente da verificação de atualizações. |

`agent_shells` é uma tabela por nome de agent, por exemplo `[agent_shells] codex = "powershell"`. `codex_windows_shell` é a forma antiga do Codex; valores conflitantes são rejeitados.

## Formatos e erros

Durações usam a sintaxe Go e podem combinar `ns`, `us`, `ms`, `s`, `m` e `h`, como `1h30m`. A retenção deve ficar entre `1` e `3650` dias. `graveyard.mode`, `ui.language` e `proposals.mode` aceitam somente os valores indicados.

Chaves desconhecidas não são ignoradas. O erro informa caminho, linha e chave completa, e sugere uma opção quando a grafia é parecida. TOML inválido também faz `obit config` falhar.

## Comandos comuns

```powershell
obit config
obit config --create
obit config --language en
obit config telemetry off
obit config telemetry on
```

`--create` escreve o modelo comentado apenas quando o arquivo não existe. `--language` aceita `auto`, `en` e `zh-CN`. `telemetry on|off` altera somente `telemetry.enabled` e não altera `[update] check`.
