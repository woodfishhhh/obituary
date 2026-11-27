# Como o Obituary funciona

## A vida de um arquivo

1. **Registrar**: atribua uma vida útil ao arquivo ou à pasta. O agente usa `obit add debug.log 2h`; você pode escolher “Add to Obituary” no menu de contexto do Explorador de Arquivos. Veja [O que é registrado](./auto-register).
2. **Expirar**: `obit list` mostra quanto tempo resta. Antes do vencimento, `obit extend` prolonga o prazo e `obit spare` cancela o acompanhamento sem excluir o arquivo.
3. **Enterrar**: quando o prazo termina, o item vai para o cemitério (`obit graveyard`), onde fica por sete dias por padrão.
4. **Ressuscitar ou excluir permanentemente**: durante a retenção, `obit resurrect <id>` devolve o item ao lugar de origem sem sobrescrever nada. A exclusão permanente (`obit purge`) só ocorre depois desse prazo.

Não sabe se ainda vai precisar de algo? Use `obit bury` para enterrá-lo agora. Durante sete dias você poderá restaurá-lo como estava.

## Onde fica o cemitério

O cemitério fica sempre na mesma unidade que o arquivo. Na unidade do diretório de dados, usa-se o cemitério desse diretório. Nas demais, usa-se a pasta oculta `.obituary` na raiz. Assim, o enterro é uma simples renomeação: é rápido e não corre o risco de mover só parte do conteúdo.

Você pode mudar o prazo de retenção nas configurações ou mandar os itens diretamente para a Lixeira do sistema. Nesse caso, restaure pela Lixeira; `obit resurrect` não se aplica:

```toml
[graveyard]
mode = "graveyard"      # or "recycle": files go to system Trash, directories to the graveyard
retention_days = 7      # days the graveyard keeps things
max_size = ""           # optional capacity limit, e.g. "5GB"; purges oldest graves first, oversized items wait for your review
```

O arquivo de configuração é `%APPDATA%\obituary\config.toml`. `obit config` mostra os ajustes e `obit config --create` cria um arquivo comentado com os valores padrão.

## Por que outros arquivos não são excluídos

- **Só a limpeza definitiva do túmulo exclui permanentemente.** Cada enterro cria uma pasta de túmulo exclusiva e registra o ID dessa pasta. Antes de excluir permanentemente ou ressuscitar, o Obituary verifica se a pasta ainda é a mesma, se contém exatamente um item e se o item é o objeto enterrado. Assim, nem um registro danificado nem um cemitério substituído por um link podem levar à exclusão de algo fora do cemitério.
- **O item só é ressuscitado no local de origem.** Se o nome estiver ocupado, nada é sobrescrito. O item volta ao lado, com um nome como `name (resurrected).ext`; os dois são mantidos e o resultado mostra o caminho real. Uma pasta pai excluída é recriada. Se uma pasta pai virou uma junção ou link simbólico, se uma junção mudou de destino ou se a letra da unidade passou a apontar para outro volume, a operação é recusada e o túmulo permanece intacto.
- **O que conta é o arquivo, não o nome.** Cada entrada guarda o número de série do volume e o ID do arquivo de 128 bits. Assim o Obituary reconhece um arquivo movido ou renomeado e não toca em outro que tenha ocupado o caminho antigo. A identidade é verificada de novo logo antes da exclusão. Arquivos com mais de um link físico são recusados.
- **Caminhos protegidos.** Raízes de unidades, `C:\Windows`, Program Files, ProgramData, perfil do usuário, Área de Trabalho, Documentos, Downloads e pastas semelhantes, bem como suas pastas pai, são protegidos, inclusive quando redirecionados. É possível atribuir vida útil ao conteúdo dessas pastas, mas não às próprias pastas.
- **Repositórios Git.** Se a pasta contiver `.git` em qualquer nível, `obit add` e `obit bury` exigem `--force`. A verificação se repete ao expirar. Se não puder ser concluída, o enterro é recusado.
- **Na dúvida, não exclui.** Uma entrada só é descartada quando há confirmação de que o arquivo não existe mais. Em caso de acesso negado, volume desconectado ou erro semelhante, o Obituary tenta novamente. Os intervalos começam em um minuto e crescem até um dia. Após 12 falhas consecutivas, ele para e aguarda sua decisão. O motivo aparece em `obit list` e em “Burials that failed” na página For you. Use `obit extend <id> 1m` (“Retry”) para tentar de novo ou `obit spare <id>` (“Stop tracking”) para deixar o item onde está. Antes de enterrar uma pasta grande, ele procura repositórios Git por até 30 segundos. Só enterra se a busca terminar sem encontrar nenhum. Se não terminar, tenta novamente mais tarde; uma pasta com repositório não é enterrada.
- **Todas as operações ficam registradas.** Cada registro, extensão, cancelamento e exclusão entra no log de auditoria (`obit log`).

## Execução em segundo plano

Com a supervisão de desktop ativa, a bandeja trata os vencimentos e uma tarefa independente verifica pendências a cada hora. Sem supervisão da bandeja para esse diretório de dados, a tarefa é executada a cada cinco minutos.

O Windows usa uma tarefa no logon, o macOS usa um LaunchAgent do usuário e o Linux usa um serviço systemd do usuário na sessão gráfica. Depois de uma falha, o sistema aguarda 60 segundos para reiniciar; na terceira falha em 15 minutos, para de tentar.

Os hooks também executam uma verificação no início e no fim da sessão.

```powershell
obit daemon enable              # tray at login plus the fallback scheduled task (--dry-run lists what would be installed)
obit daemon status              # current mode and the next expiry (--json)
obit daemon disable             # remove the scheduled task and login start, and stop the running daemon (--dry-run previews)
```

Consultas como `list`, `graveyard`, `stats` e `log` são somente leitura e nunca enterram nada.

## Onde ficam os dados {#where-the-data-lives}

`%LOCALAPPDATA%\obituary` contém `store.json` (registro), `audit.jsonl` (log de auditoria), `obitd.log` (log do daemon) e `graveyard\` (cemitério desta unidade).

Para mudar os locais, use `OBIT_HOME` (dados), `OBIT_CONFIG_DIR` (configuração) e `OBIT_USER_HOME` (pasta pessoal do usuário).

Ao abrir novamente com a mesma pasta de dados, a janela existente volta ao primeiro plano. Outro `OBIT_HOME` abre uma janela separada. Se a pasta não for a padrão, o caminho aparece no título.
