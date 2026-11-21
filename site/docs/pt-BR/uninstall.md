# Desinstalação

```powershell
obit uninstall --dry-run      # see what would be removed
obit uninstall                # remove everything Obituary set up, after you confirm
.\install.ps1 -Uninstall      # if you used the install script: also remove the program files, PATH entry and Start menu shortcut
```

`obit uninstall` remove apenas o que o Obituary adicionou: hooks dos agentes, MCP e Skill instalados para o usuário e para o projeto Git atual, inicialização automática no login, tarefa agendada, daemon em execução, menu de contexto, datas exibidas no Explorador de Arquivos e políticas de pastas.

## O cemitério e o registro são preservados

Por padrão, o cemitério e o registro são preservados. O comando informa onde fica o cemitério e quanto há nele. Se quiser excluir os dois, acrescente `--purge-data`. O comando lista o conteúdo do cemitério e pede uma nova confirmação.

## Hooks em outros projetos

Se você também instalou hooks locais em outros projetos, execute `obit uninstall` em cada um antes de remover o programa. Caso contrário, esses hooks vão parar de funcionar quando não encontrarem mais `obit`.

## Scoop e winget

Execute primeiro `obit uninstall` e depois o comando de desinstalação do gerenciador de pacotes que usou.
## Escopo da desinstalação e dados

`obit uninstall --dry-run` lista itens removíveis para o usuário e o projeto Git atuais. O comando remove hooks, MCP, Skills, bandeja, inicialização e agendadores, mas não altera outros projetos; execute-o em cada um deles.

| Item | Windows | macOS | Linux |
| --- | --- | --- | --- |
| Programa | `obit`, `obitd`, `obit-gui` opcional | `/Applications/Obituary.app` ou diretório CLI | Diretório CLI; desktop em `.deb` |
| Hook, MCP, Skill | Configuração do agent | Configuração do agent | Configuração do agent |
| Agendamento | Tarefa, processo residente e bandeja | LaunchAgent e barra de menus | Timer systemd e XDG autostart |
| Projeto | Partes adicionadas a `.claude/` e `.codex/` | Configuração do projeto | Configuração do projeto |

Os dados padrão ficam em `%LOCALAPPDATA%\obituary` e `%APPDATA%\obituary` no Windows, `~/Library/Application Support/obituary` no macOS e `$XDG_DATA_HOME/obituary` no Linux (`~/.local/share/obituary` sem a variável). `OBIT_HOME`, `OBIT_CONFIG_DIR` e `OBIT_USER_HOME` redirecionam esses locais; `.obituary` também pode existir no mesmo volume.

Cemitério, registro, auditoria e configuração são preservados. `--purge-data` remove apenas dados próprios e verificados do cemitério, mantendo arquivos desconhecidos. No modo recycle, restaure pela lixeira, não por `resurrect`.
