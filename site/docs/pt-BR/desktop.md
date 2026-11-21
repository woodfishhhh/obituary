# Janela, bandeja e menu de contexto

Você pode ressuscitar arquivos, aprovar propostas e alterar as configurações sem usar a linha de comando.

## Janela pequena e ícone da barra de menus no macOS

Baixe o DMG universal do macOS e o arquivo de checksums da mesma versão. Verifique o SHA-256, abra o DMG e arraste Obituary.app para Aplicativos ou para a pasta Applications do usuário; ejete o DMG antes de ativar a inicialização no login.

O app contém a janela Wails, o ícone da barra de menus e os auxiliares obit e obitd. O agente de usuário do launchd que verifica vencimentos a cada 5 minutos continua sendo um fallback separado. Em Configurações, ative o início da bandeja no login para instalar o LaunchAgent do Obituary; desativar remove somente essa entrada.

Remover `Obituary.app` mantém seus dados no diretório padrão `~/Library/Application Support/obituary/`: `store.json` contém o registro, `config.toml` guarda as configurações e `graveyard/` contém os arquivos enterrados nesse volume.

### Primeira abertura de um app sem assinatura

Esta versão não tem assinatura Developer ID nem notarização. Confirme a origem e o SHA-256 e tente abrir uma vez. Se o macOS não puder verificar o desenvolvedor, abra « Ajustes do Sistema → Privacidade e Segurança », escolha « Abrir Mesmo Assim » para Obituary, confirme e abra novamente. Não desative o Gatekeeper globalmente nem remova a quarentena de forma recursiva. Pare diante de avisos de app danificado, malware ou política da organização.

### Proposta de Homebrew Cask

O cask deve instalar o mesmo DMG universal, copiar Obituary.app para Aplicativos e disponibilizar obit e obitd no app bundle. Até que o DMG de desktop tenha uma URL estável e uma verificação de desinstalação revisada, ele fica separado do cask existente para CLI.
## Bandeja do sistema

`obit init` configura o ícone da bandeja para aparecer no login. No menu, você pode:

- ver os itens enterrados recentemente e clicar em um deles para ressuscitá-lo;
- consultar itens que expiram nas próximas 24 horas;
- pausar ou retomar a limpeza automática;
- abrir a janela com “Open Obituary” (uma notificação explica o problema se `obit-gui.exe` não estiver na mesma pasta que `obitd.exe`);
- sair da bandeja sem interromper a tarefa agendada.

## A pequena janela

`obit-gui.exe` tem cinco páginas: Upcoming, Graveyard, Decisions, Settings e Logs. A entrada “Obituary” no menu Iniciar, o atalho da área de trabalho e “Open Obituary” na bandeja abrem a mesma janela.

- A barra superior da página inicial mostra “Auto registration: off” ou “on”. Clique nela para abrir as configurações. No topo de Decisions ou em Settings você pode desativar o registro (padrão: só entra o que você e o agente registram), usar o modo em camadas, deixar tudo automático ou perguntar sempre. Veja [O que é registrado](./auto-register). A seção “Handled automatically”, mais abaixo, lista os itens registrados sem perguntar. Um clique desfaz o registro (a contagem para e o item fica no lugar) ou estende a vida útil.
- Se um item expirar e não puder ser enterrado, por exemplo porque outro programa mantém o arquivo aberto, ele aparecerá em “Burials that failed” na página Decisions, com o motivo e a hora da próxima tentativa. O Obituary aumenta o intervalo entre as tentativas. Depois de repetidas falhas, ele espera sua decisão: “Retry” tenta de novo agora; “Stop tracking” encerra o acompanhamento e deixa o item no lugar.
- Com o Docker parado, o botão de aprovação de uma proposta de contêiner fica indisponível. O cartão explica o motivo. Você poderá aprovar quando o Docker iniciar.
- Ao confiar em um arquivo `.obitrules`, você aprova exatamente a versão mostrada na janela.
- A janela, a bandeja e o menu de contexto seguem `[ui] language` em `config.toml` (`auto`, `zh-CN` ou `en`; `auto` usa o idioma do Windows). Você também pode alterar o idioma nas configurações.
- A janela só está disponível em x64 e exige o runtime do WebView2. Ele já faz parte do Windows 11 e está presente na maioria dos computadores com Windows 10.

### Actualizações

Para instalar uma versão estável mais recente, abra Settings → About and updates. O Obituary fecha a janela e a bandeja, atualiza pelo mesmo canal usado na instalação (Scoop, WinGet ou script) e tenta abrir a bandeja e a janela novamente. A mesma página exibe eventuais falhas e os caminhos dos logs. Se você extraiu o programa manualmente, baixe a nova cópia na página de versões.

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

Para omitir esses itens na instalação, use `obit init --no-context-menu` para o menu de contexto e `--no-shortcuts` para os atalhos do menu Iniciar e da área de trabalho.

Veja [Políticas de pastas](./folder-policies#dates-in-explorer) para saber como as datas de enterro aparecem no Explorador.
