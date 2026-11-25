# Desinstalação

Desinstalar tem duas etapas: primeiro o Obituary remove o que configurou, depois você apaga os programas em si. O cemitério e o registro ficam por padrão, de modo que os arquivos que você enterrou ainda podem ser ressuscitados depois.

```powershell
obit uninstall --dry-run      # lista o que seria removido; não altera nada
obit uninstall                # mostra o plano e o aplica depois que você confirmar
```

Se você instalou pelo script no Windows, um comando basta:

```powershell
.\install.ps1 -Uninstall      # executa obit uninstall --yes por você e depois remove os arquivos do programa, a entrada do PATH que ele adicionou e seu atalho do menu Iniciar
```

## O que é removido

`obit uninstall` trabalha no nível do usuário e no projeto Git atual: os hooks, o MCP e a Skill de cada agente, a tarefa agendada (o Agendador de Tarefas no Windows, um agente de usuário do launchd no macOS, um timer de usuário do systemd no Linux), o processo residente e o ícone da bandeja, e as políticas de pastas (os arquivos sob uma política ficam onde estão). No Windows, também remove a entrada de inicialização no login, o menu de contexto do Explorador de Arquivos e as etiquetas de data do Explorador.

No macOS e no Linux, a desinstalação também remove as ações que o Obituary instalou no Finder, Nautilus e Dolphin. Os scripts do usuário e as etiquetas que o Obituary não adicionou são preservados.

Ele só mexe no que o próprio Obituary escreveu. Uma entrada que você editou, ou que outra instalação deixou, é mantida como está, e o plano diz qual e por quê. Onde não há terminal (um script, a linha de comando de um agente), acrescente `--yes`; sem ele, o comando para e avisa que precisa de um terminal.

Há três coisas que ele não faz por você. Os arquivos do programa são apagados conforme o modo como você instalou (veja abaixo). Se você instalou hooks no nível de projeto em outros projetos, execute `obit uninstall` em cada um deles antes de remover os programas, ou os hooks de lá vão falhar porque `obit` não existirá mais. E, no macOS e no Linux, a entrada de inicialização da bandeja não está na lista: antes de desinstalar, desative “Start the tray at sign-in” em Settings na janela, o que remove a entrada que o Obituary escreveu. No Windows, `obit uninstall` cuida da inicialização da bandeja, mas os atalhos do menu Iniciar e da área de trabalho criados por `obit init` não estão na lista dele; `obit init --uninstall` os remove, a menos que você acrescente `--no-shortcuts`.

## O cemitério, o registro e sua configuração

Três coisas ficam por padrão: os arquivos do cemitério, o registro com o log de auditoria e o `config.toml`. `obit uninstall` diz quanto ainda há no cemitério e onde ele está, e uma reinstalação posterior retoma tudo. Se você usa o modo Lixeira, o que expirou já está na Lixeira do sistema; a desinstalação não mexe nela, e a restauração é feita a partir dela, porque `obit resurrect` não a alcança.

Quando você realmente quiser apagar tudo, acrescente `--purge-data`:

```powershell
obit uninstall --purge-data
```

Ele lista primeiro cada item do cemitério e pede mais uma confirmação depois que você confirma o plano de desinstalação. Depois disso, e antes de remover qualquer coisa, o Obituary verifica tudo o que vai tocar: se o diretório de dados é dele, se reconhece cada arquivo que há nele e se consegue verificar a pasta do cemitério e cada sepultura a ser limpa. Se alguma verificação falhar, ele para ali e diz qual; o cemitério, o registro e os serviços ficam como estavam, então uma limpeza nunca fica pela metade. Só quando tudo passa ele apaga esses arquivos de forma permanente e, em seguida, o registro, o log de auditoria e os demais arquivos que o Obituary mantém. Em um script, `--yes --purge-data` juntos pulam as duas perguntas.

Ele só apaga arquivos que reconhece. Outros arquivos, links ou uma subpasta que o Obituary não conhece fazem a verificação parar, e ele diz qual encontrou. O `config.toml` nunca faz parte da limpeza e você não precisa movê-lo, nem quando ele está na mesma pasta que o registro (o padrão no macOS): depois da limpeza, essa pasta contém apenas o `config.toml`. Quando a configuração está em outro lugar, o diretório de dados inteiro é apagado.

## Remover os programas

Como remover os programas depende de como você os instalou.

| Instalado com | Remover com |
| --- | --- |
| O script de instalação do Windows | `.\install.ps1 -Uninstall` |
| Scoop | `obit uninstall` e depois `scoop uninstall obituary` |
| winget | `obit uninstall` e depois `winget uninstall woodfishhhh.Obituary` |
| A janela do macOS (DMG) | `obit uninstall` e desativar “Start the tray at sign-in”; depois arrastar o Obituary.app de Aplicativos para o Lixo |
| Homebrew | `obit uninstall` e depois `brew uninstall --cask obituary` |
| `install.sh` no macOS ou no Linux | `obit uninstall` e depois apagar `obit` e `obitd` de `~/.local/bin` |
| O pacote de desktop do Ubuntu (`.deb`) | desativar “Start the tray at sign-in” e sair da bandeja; depois `sudo apt remove obituary-desktop` |

O app do macOS e o `.deb` do Ubuntu trazem cada um o seu `obit`, mas nenhum deles o coloca no seu PATH. Se você não instalou a linha de comando separadamente, execute-o pelo caminho completo antes de desinstalar: `/Applications/Obituary.app/Contents/MacOS/obit` no app, `/usr/libexec/obituary/obit` no `.deb`. `apt remove` apaga os programas dessa pasta e deixa seu cemitério, suas configurações e qualquer linha de comando que você tenha instalado de outro jeito.

## Onde ficam os dados

| | Dados (registro, cemitério, logs) | Configuração (`config.toml`) |
| --- | --- | --- |
| Windows | `%LOCALAPPDATA%\obituary` | `%APPDATA%\obituary` |
| macOS | `~/Library/Application Support/obituary` | a mesma pasta |
| Linux | `~/.local/share/obituary` | `~/.config/obituary` |

No Linux, se `XDG_DATA_HOME` ou `XDG_CONFIG_HOME` estiverem definidas, cada uma é a pasta `obituary` dentro dela. As variáveis de ambiente `OBIT_HOME` e `OBIT_CONFIG_DIR` levam os dados e a configuração para outro lugar, e `obit config` mostra quais estão em uso. No disco que contém o diretório de dados, o cemitério é a pasta `graveyard` dele; nos outros discos, é uma pasta oculta `.obituary` na raiz do disco, para que enterrar seja sempre uma renomeação dentro de um mesmo disco e nunca uma cópia.
