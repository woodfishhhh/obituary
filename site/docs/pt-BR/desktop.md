# Janela, bandeja e menu de contexto

Você pode ressuscitar arquivos, aprovar propostas e alterar as configurações sem usar a linha de comando.

## O que cada uma das três plataformas oferece

A janela, o ícone da bandeja e as notificações de desktop existem no Windows, no macOS e no Ubuntu 24.04; o menu de contexto e as etiquetas de data do Explorador de Arquivos existem só no Windows. As edições de desktop para macOS e Ubuntu são, por enquanto, versões de pré-lançamento. Outras distribuições Linux não têm edição de desktop e usam a linha de comando.

| | Windows | macOS | Ubuntu 24.04 |
| --- | --- | --- | --- |
| Janela | `obit-gui.exe`, só x64 | `Obituary.app`, universal para Intel e Apple Silicon | um pacote `.deb`, só amd64 |
| Ícone | área de notificação | barra de menus | ícone na bandeja; exige um desktop que ofereça StatusNotifierItem |
| Ícone no login | ativado por padrão após `obit init` | interruptor em Settings | interruptor em Settings |
| Verificação de expiração | processo residente do ícone | LaunchAgent | systemd |
| Notificações de desktop | sim | sim | sim |
| Menu de contexto | no Explorador de Arquivos | não | não |
| Etiquetas de data no Explorador | sim | não | não |
| Modo Lixeira | sim | não, só cemitério | sim |
| Instalado com | o script de instalação, Scoop, winget ou o arquivo compactado | o DMG | o `.deb` |
| Primeira abertura | um arquivo baixado à mão pode gerar um aviso do SmartScreen | exige aprovação manual, veja [abaixo](#primeira-abertura-de-um-app-sem-assinatura) | abre direto |
| Requisitos | o runtime do WebView2 | a 1.6.0 exige macOS 26 (corrigido na 1.6.1) | GTK 3 e WebKitGTK 4.1, instalados pelo apt |

O Windows usa uma tarefa no logon, o macOS usa um LaunchAgent do usuário e o Linux usa um serviço systemd do usuário na sessão gráfica. Depois de uma falha, o sistema aguarda 60 segundos para reiniciar; na terceira falha em 15 minutos, para de tentar.

Com a supervisão de desktop ativa, a bandeja trata os vencimentos e uma tarefa independente verifica pendências a cada hora. Sem supervisão da bandeja para esse diretório de dados, a tarefa é executada a cada cinco minutos.

No Windows, uma tarefa antiga só é migrada se corresponder à definição criada para essa instalação. O original é guardado para reversão; tarefas alteradas ou desativadas permanecem como estão.

## Janela pequena e ícone da barra de menus no macOS

Baixe o DMG universal (`obituary_<versão>_darwin_universal.dmg`) e o `checksums.txt` da página da versão, confira o SHA-256, abra o DMG, arraste o Obituary.app para Aplicativos, ejete o DMG e abra o app a partir de Aplicativos. Arraste primeiro porque a entrada de inicialização no login guarda onde o app está, e um app executado direto do DMG deixa de estar lá quando o DMG é ejetado.

O app contém a janela, o ícone da barra de menus e os programas `obit` e `obitd` que o acompanham. Ativar “Start the tray at sign-in” em Settings na janela instala o LaunchAgent de usuário do próprio Obituary, para que o ícone da barra de menus apareça quando você fizer login; desativá-lo remove apenas essa entrada.

Instale a tarefa independente com `obit daemon enable`. Fechar a janela ou sair pelo ícone da barra de menus não desativa a tarefa.

Remover `Obituary.app` mantém seus dados no diretório padrão `~/Library/Application Support/obituary/`: `store.json` contém o registro, `config.toml` guarda as configurações e `graveyard/` contém os arquivos enterrados nesse volume.

### Primeira abertura de um app sem assinatura

A versão não tem assinatura Developer ID nem notarização, então o macOS a barra na primeira vez. Depois de confirmar que o download veio de uma página de versões confiável e de conferir o SHA-256, abra o app normalmente uma vez. Se o sistema disser que não consegue verificar o desenvolvedor, abra Ajustes do Sistema, depois Privacidade e Segurança, encontre Obituary no fim da página, escolha “Abrir Mesmo Assim”, confirme mais uma vez e abra o app de novo. Não desative o Gatekeeper globalmente por causa disso nem apague o atributo de quarentena de forma recursiva. Se o sistema informar que o app está danificado ou contém malware, ou que uma política da empresa proíbe abri-lo, pare por aí.

### A 1.6.0 exige macOS 26

A janela do macOS na 1.6.0 exige o macOS 26. No macOS 15 e anteriores, o sistema diz que o app não é compatível com esta versão e ele não abre. Isso está corrigido na 1.6.1.

## Ubuntu 24.04 (amd64)

Baixe o pacote de desktop `obituary_<versão>_linux_amd64.deb` e o `checksums.txt` da página da versão. No diretório de download, execute os comandos abaixo, trocando `VERSION` pela versão que você baixou; o pacote só é instalado se a verificação for bem-sucedida:

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict - && sudo apt install "./$file"
```

Depois, abra o Obituary pela lista de aplicativos. A janela usa GTK 3 e WebKitGTK 4.1, e o apt os instala junto com o pacote. Os programas ficam em `/usr/libexec/obituary/`, que não está no seu PATH; para usar `obit` em um terminal, instale a linha de comando separadamente com o script de instalação.

A bandeja precisa de um desktop que ofereça StatusNotifierItem, e o GNOME não oferece por padrão. Ali você abre a mesma janela pela lista de aplicativos, sem nenhuma extensão, mas não há ícone na bandeja. Para que o ícone apareça no seu próximo login, ative “Start the tray at sign-in” em Settings; desativá-lo apaga a entrada de inicialização que o Obituary escreveu. Para desinstalar, veja [Desinstalação](./uninstall).

## Bandeja do sistema

`obit init` configura o ícone da bandeja para aparecer no login. No menu, você pode:

- ver os itens enterrados recentemente e clicar em um deles para ressuscitá-lo;
- consultar itens que expiram nas próximas 24 horas;
- pausar ou retomar a limpeza automática;
- abrir a janela com “Open Obituary” (uma notificação explica o problema se `obit-gui.exe` não estiver na mesma pasta que `obitd.exe`);
- sair da bandeja sem interromper a tarefa agendada.

## A pequena janela

`obit-gui.exe` tem cinco páginas: Expiring, Graveyard, For you, Settings e Logs. A entrada “Obituary” no menu Iniciar, o atalho da área de trabalho e “Open Obituary” na bandeja abrem a mesma janela.

- A barra superior da página inicial mostra “Auto registration: off” ou “on”. Clique nela para abrir as configurações. No topo de For you ou em Settings você pode desativar o registro (padrão: só entra o que você e o agente registram), usar o modo em camadas, deixar tudo automático ou perguntar sempre. Veja [O que é registrado](./auto-register). A seção “Handled automatically”, mais abaixo, lista os itens registrados sem perguntar. Um clique desfaz o registro (a contagem para e o item fica no lugar) ou estende a vida útil.
- Se um item expirar e não puder ser enterrado, por exemplo porque outro programa mantém o arquivo aberto, ele aparecerá em “Burials that failed” na página For you, com o motivo e a hora da próxima tentativa. O Obituary aumenta o intervalo entre as tentativas. Depois de repetidas falhas, ele espera sua decisão: “Retry” tenta de novo agora; “Stop tracking” encerra o acompanhamento e deixa o item no lugar.
- Com o Docker parado, o botão de aprovação de uma proposta de contêiner fica indisponível. O cartão explica o motivo. Você poderá aprovar quando o Docker iniciar.
- Ao confiar em um arquivo `.obitrules`, você aprova exatamente a versão mostrada na janela.
- A janela, a bandeja e o menu de contexto seguem `[ui] language` em `config.toml` (`auto`, `zh-CN` ou `en`; `auto` usa o idioma do Windows). Você também pode alterar o idioma nas configurações.
- A janela só está disponível em x64 e exige o runtime do WebView2. Ele já faz parte do Windows 11 e está presente na maioria dos computadores com Windows 10.

### Actualizações

Para instalar uma versão estável mais recente, abra Settings → About and updates. O Obituary fecha a janela e a bandeja, atualiza pelo mesmo canal usado na instalação (Scoop, WinGet ou script) e tenta abrir a bandeja e a janela novamente. A mesma página exibe eventuais falhas e os caminhos dos logs.

Uma cópia que você extraiu manualmente também é atualizada no lugar; o botão diz “Download and restart to update”. O Obituary baixa a versão do GitHub (do espelho da China continental se o GitHub não responder em dez segundos), confere o SHA-256 e só então fecha a janela e substitui os programas do próprio Obituary nessa pasta (`obit`, `obitd` e `obit-gui`). Os demais arquivos da pasta ficam como estão, e PATH e o menu Iniciar não são alterados. Os programas anteriores ficam um dia em uma pasta oculta `.obituary-update-…` no mesmo lugar e depois vão para o cemitério, de onde ainda podem ser ressuscitados; se qualquer etapa falhar, inclusive um novo `obit` que não inicia, a versão anterior é restaurada. É preciso uma versão estável (não uma compilação de desenvolvimento), todos os programas da versão na pasta como arquivos comuns e uma pasta em que o usuário atual possa gravar sem permissões de administrador. Se alguma condição não for atendida, a página informa qual, e você ainda pode baixar a nova cópia na página de versões. No terminal, `obit update --apply` faz o mesmo. “What changed” mostra as notas no idioma da janela, ou em inglês quando não há tradução.

### Oito temas

Escolha o tema em Settings → Appearance. A mudança é imediata e afeta apenas a aparência da janela neste computador.

| Tema | Descrição |
|---|---|
| Calendário | Uma coluna por dia. Cada arquivo aparece na data do enterro ou da exclusão permanente. Arraste-o para uma data posterior para prolongar a vida útil. |
| Uma linha | Uma única linha de entrada, só pelo teclado: encontre o arquivo e pressione Enter para ressuscitá-lo ou prolongar a vida útil. |
| Túnel | Um túnel do tempo em 3D. Quanto mais perto o arquivo, mais cedo ele expira. Role para percorrer as datas. |
| Vigília | Uma luz por arquivo sobre fundo escuro. Quanto mais fraca a luz, mais próximo o vencimento. |
| Obituários | Página de obituários de jornal, com colunas em fonte serifada e um carimbo ao ressuscitar um arquivo. |
| Horizonte | Arquivos monitorados acima do solo, cemitério abaixo. |
| Mica | Visual nativo do Windows 11 com barra lateral. |
| Noite | Tema escuro e discreto com barra superior. |

## Menu de contexto

`obit init` instala por padrão o menu de contexto do Explorador de Arquivos, apenas para o usuário atual. Clique com o botão direito em um arquivo ou pasta, escolha “Add to Obituary” e selecione:

- **1 dia / 7 dias / 30 dias**: o item é enterrado quando o prazo termina. Você pode usá-lo normalmente até lá.
- **Bury now (recoverable)**: envia o item ao cemitério imediatamente. É possível ressuscitá-lo durante o período de retenção (7 dias por padrão).

Você pode selecionar vários arquivos e adicioná-los de uma vez. A notificação informa quantos entraram, quando serão enterrados e quantos falharam. O novo menu do Windows 11 não mostra essas opções: clique em “Mostrar mais opções” ou mantenha Shift pressionado ao clicar com o botão direito. `obit init --dry-run` lista cada entrada com o comando correspondente; `obit doctor` verifica se a instalação está correta.

Se após uma atualização todos os rótulos do menu do Explorer estiverem em outro idioma compatível, o menu continua funcionando. `obit doctor` mostra um aviso informativo (JSON: `installation: label-language-mismatch`). Execute `obit init --context-menu --yes` para aplicar o idioma atual da interface.

O macOS e o Linux ainda não têm menu de contexto. Use `obit mark <path> 7d` para registrar um item ou `obit bury <path>` para enterrá-lo agora.

## Ícones e menus

`obit init` configura todos os itens abaixo por padrão. Cada um tem seu próprio controle em Settings → Icons and menus:

| Opção | O que controla |
|---|---|
| Ícone da bandeja | Obituary na área de notificação. A limpeza continua mesmo com o ícone oculto. |
| Ícone da barra de tarefas | Se a janela ocupa um espaço na barra de tarefas enquanto está aberta. |
| Menu Iniciar | Entrada “Obituary” no menu Iniciar. |
| Atalho da área de trabalho | “Obituary” na área de trabalho. |
| Menu de contexto | Entrada “Add to Obituary” no Explorador de Arquivos. |

Se você não os quiser na instalação, acrescente `obit init --no-context-menu` para omitir o menu de contexto ou `--no-shortcuts` para omitir os atalhos do menu Iniciar e da área de trabalho. As mesmas opções valem na desinstalação: `obit init --uninstall --no-shortcuts` mantém os atalhos existentes e remove o restante como de costume, e `--no-context-menu` faz o mesmo com o menu de contexto.

Veja [Políticas de pastas](./folder-policies#dates-in-explorer) para saber como as datas de enterro aparecem no Explorador.
