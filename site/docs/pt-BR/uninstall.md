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
