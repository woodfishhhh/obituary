# Solução de problemas

Quando algo der errado, comece por `obit doctor`. Ele só lê e nunca escreve. Verifica a instalação, as integrações com agentes, o agendamento em segundo plano e seus dados, e cada linha começa com `PASS`, `INFO`, `WARN` ou `FAIL`, seguida do motivo e do que fazer; havendo algum `FAIL`, o comando termina com código de saída diferente de zero. Acrescente `--json` para repassar o resultado a outra pessoa, ou `--verbose` para ver exatamente quais recursos cada agente tem.

Cada seção abaixo segue a mesma ordem: o que você vê, por que acontece, o que fazer e como saber que foi resolvido.

## O shell não encontra `obit`

O terminal diz `command not found`, ou o PowerShell não reconhece `obit`. O programa está no disco, mas a pasta dele não está no seu PATH. O script de instalação do Windows adiciona `%LOCALAPPDATA%\Programs\obituary` ao PATH do seu usuário, mas um terminal que já estava aberto não enxerga o PATH novo. O `install.sh` coloca os programas em `~/.local/bin` e nunca edita o PATH, e em alguns sistemas essa pasta nem está no PATH de início. O `.deb` do Ubuntu os coloca em `/usr/libexec/obituary/`, que também não está no PATH.

No Windows, feche o terminal e abra um novo. No macOS e no Linux, adicione `~/.local/bin` à configuração do seu shell: no zsh, coloque `export PATH="$HOME/.local/bin:$PATH"` em `~/.zshrc` e abra um terminal novo. Se você instalou só o `.deb`, use o caminho completo `/usr/libexec/obituary/obit`, ou instale a linha de comando separadamente com o script de instalação.

Está resolvido quando `obit --version` mostra uma versão e a linha `path` do `obit doctor` deixa de ser `WARN`.

## `obit doctor` não encontra `obitd`

Uma linha `FAIL obitd` no `obit doctor` significa que o programa em segundo plano `obitd` não está ao lado de `obit`. O Obituary o procura na mesma pasta, então copiar só o `obit` não basta. Deixe `obit` e `obitd` juntos, ou execute o instalador de novo, do jeito que você instalou. Quando o `obit doctor` mostrar `PASS` nessa linha, está resolvido.

## Um arquivo baixado não passa na soma de verificação

Se você usa o script de instalação, ele mesmo faz essa verificação e para sem instalar nada se o hash não bater. Você só precisa conferir à mão quando baixou por conta própria um arquivo compactado, um DMG ou um `.deb`. Uma divergência costuma significar que o download está incompleto, que o arquivo foi alterado no caminho ou que o `checksums.txt` é de outra versão, já que os hashes são de cada versão.

Baixe de novo o pacote e o `checksums.txt` da mesma página de versão (do espelho, se você estiver na China continental) e confira outra vez; se ainda não bater, não execute. Troque `VERSION` e o nome do arquivo pelos do que você baixou. No Linux:

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict -
```

No macOS, troque `sha256sum --check --strict -` da última linha por `shasum -a 256 -c -`. No Windows PowerShell:

```powershell
$file = "obituary_VERSION_windows_amd64.zip"
(Get-FileHash $file -Algorithm SHA256).Hash -eq ((Select-String -Path checksums.txt -SimpleMatch $file).Line -split '\s+')[0]
```

Quando os hashes coincidem, Linux e macOS imprimem o nome do arquivo seguido de `OK`, e o PowerShell imprime `True`.

## O Windows mostra “O Windows protegeu o computador”

O Windows SmartScreen pode barrar um arquivo compactado que você baixou à mão pelo navegador, porque os programas do Obituary ainda não têm assinatura de código. Com Scoop, winget ou o script de instalação, isso raramente acontece. Primeiro confira o SHA-256 como descrito acima; se ele bater, escolha “Mais informações” e depois “Executar assim mesmo”. Se você não conseguir conferir, ou se o Windows relatar malware e não apenas um editor desconhecido, não execute.

## A janela do Windows não abre

Clicar em Obituary no menu Iniciar ou na bandeja não faz nada, ou aparece um aviso de que falta um componente de execução. A janela é desenhada pelo runtime Microsoft Edge WebView2. O Windows 11 já o inclui e a maioria das máquinas com Windows 10 também o tem, mas onde ele falta, a janela não consegue iniciar. Baixe e instale o runtime do WebView2 no site da Microsoft (a versão Evergreen) e clique em Obituary de novo; quando a janela abrir, está resolvido.

A janela existe por enquanto apenas para x64. O arquivo do Windows para ARM64 não inclui `obit-gui.exe`, então ali você usa a linha de comando. Se “Open Obituary” no menu da bandeja mostrar uma notificação dizendo que não encontra o programa da janela, é porque `obit-gui.exe` não está ao lado de `obitd.exe`, que é exatamente como o arquivo ARM64 é montado. No x64, coloque os dois na mesma pasta.

## macOS: o app diz que não é compatível com esta versão

A janela do macOS na 1.6.0 (`Obituary.app` dentro do DMG) exige o macOS 26. No macOS 15 e anteriores, ao abri-la aparece uma mensagem de que o app não é compatível com esta versão do macOS, e ele não inicia. É um erro de empacotamento da 1.6.0, corrigido na 1.6.1. Até lá, a janela exige o macOS 26. O problema está apenas na janela que vem no DMG. Quando você tiver a 1.6.1 ou posterior, o app abre, e é assim que você saberá.

## macOS: “não foi possível verificar o desenvolvedor”

O app do DMG não tem assinatura Developer ID nem notarização, por isso o Gatekeeper o barra na primeira vez. Depois de conferir o SHA-256, abra-o normalmente uma vez. Quando o macOS disser que não consegue verificar o desenvolvedor, vá a Ajustes do Sistema, depois Privacidade e Segurança, encontre Obituary no fim da página, escolha “Abrir Mesmo Assim” e confirme de novo. Os passos estão em [Primeira abertura de um app sem assinatura](./desktop#primeira-abertura-de-um-app-sem-assinatura). Não desative o Gatekeeper globalmente por causa disso. Se o macOS disser que o app está danificado ou contém malware, é outra coisa, e você não deve permitir.

## A bandeja não inicia no login (macOS, Ubuntu)

Depois do login, não há ícone do Obituary na barra de menus nem na bandeja. A inicialização no login depende de uma entrada de inicialização: no macOS, o LaunchAgent de usuário `site.woodfish.obituary.tray`; no Ubuntu, `~/.config/autostart/obituary.desktop`. Se o interruptor em Settings na janela estiver desativado, ou o app tiver sido movido, ou o interruptor tiver sido ativado enquanto o app rodava direto do DMG, o caminho gravado na entrada ficou obsoleto e nada é encontrado no login.

Abra a janela e, em Settings, desative e ative de novo “Start the tray at sign-in”, para que o Obituary reescreva a entrada. No macOS, `launchctl list | grep obituary` deve agora mostrar `site.woodfish.obituary.tray`; no Ubuntu, `ls ~/.config/autostart/obituary.desktop` deve encontrar o arquivo, e o ícone aparecerá no seu próximo login.

## Ubuntu: a janela não abre, ou não há ícone na bandeja

Clicar em Obituary na lista de aplicativos não faz nada, o que costuma significar que falta o GTK 3 ou o WebKitGTK 4.1. O `apt install ./….deb` os instala para você; eles só faltam se foram removidos depois ou se a instalação foi interrompida no meio. No Ubuntu 24.04 o pacote do GTK 3 se chama `libgtk-3-0t64`; o nome sem `t64`, `libgtk-3-0`, não existe nesta versão, então um comando copiado de um guia antigo falha:

```bash
sudo apt update
sudo apt install libgtk-3-0t64 libwebkit2gtk-4.1-0 xdg-utils
```

Abra o Obituary de novo pela lista de aplicativos; quando a janela aparecer, está resolvido. Se a janela abre mas não há ícone na bandeja, é porque a bandeja precisa de um desktop que ofereça um host de StatusNotifierItem, e o GNOME não faz isso por padrão. Você pode abrir a mesma janela pela lista de aplicativos sem nenhuma extensão; para ter o ícone, adicione ao GNOME uma extensão que o suporte, como o AppIndicator.

## O que expirou não está sendo enterrado

A vida útil de um arquivo acabou há muito tempo e ele continua onde estava, e a linha `runner:` de `obit daemon status` diz `off`. A expiração é tratada por uma tarefa agendada que roda a cada 5 minutos: “Obituary Runner” no Agendador de Tarefas do Windows, um agente de usuário do launchd no macOS, o timer de usuário do systemd `obituary.timer` no Linux. Se ela não estiver instalada, tiver sido desligada ou tiver sido instalada para outro diretório de dados, ninguém age quando chega a hora; nesse caso, o que expirou espera o próximo `obit sweep`, ou o hook de um agente, para ser tratado.

Execute `obit daemon enable` para instalar a tarefa (no Windows isso também instala a bandeja no login) e `obit sweep` se quiser a limpeza agora. Num Linux sem serviço de usuário do systemd, como em alguns contêineres e sistemas mínimos, `obit daemon enable` não consegue instalá-la; use o cron para executar `obitd --once --home <diretório de dados> --config-dir <diretório de configuração>` a cada 5 minutos, e `obit config` informa os dois diretórios. Quando a linha `runner` de `obit daemon status` deixar de dizer `off`, está resolvido; no Linux você também pode ver o timer com `systemctl --user list-timers | grep obituary`.

## Algo expirou, mas não foi enterrado

O item aparece na página “For you” da janela, em “Burials that failed”, com um motivo ao lado. Muitas vezes um arquivo está aberto em um editor, em um cliente de sincronização ou em outro programa, e continua onde estava. Enterrar é mover o arquivo, e isso falha quando ele está em uso ou falta permissão. O Obituary não força: o original fica intacto, ele tenta de novo depois de um tempo, esperando mais a cada vez, e depois de várias falhas para e espera por você.

Feche o que está segurando o arquivo e clique em “Retry”, ou execute `obit sweep`. Se você não quiser mais que ele cuide disso, clique em “Stop tracking”: o arquivo fica onde está e a contagem regressiva termina. Quando o item some de “Burials that failed” e aparece em `obit graveyard`, ele foi enterrado.

## Um arquivo ressuscitado tem outro nome

`obit resurrect` não deu erro, mas o arquivo voltou como `report (resurrected).txt`, porque já havia algo com esse nome no lugar original. Ressuscitar nunca sobrescreve; quando o nome está ocupado, o arquivo volta para a mesma pasta com outro nome, e as duas cópias ficam intactas. Compare-as, fique com a que quiser e renomeie.

## O arquivo de configuração tem um erro

`obit config` diz `config file has errors`, seguido de um caminho de arquivo, um número de linha e um nome de chave. O `config.toml` contém uma chave que o Obituary não conhece ou um valor não permitido. O Obituary não ignora isso em silêncio, porque então `retention = 30` deixaria em vigor a retenção de 7 dias enquanto você achava que ela tinha mudado. Corrija a grafia, as aspas ou a duração na linha indicada e execute `obit config` de novo; quando ele parar de reclamar e a linha `config` do `obit doctor` for `PASS`, está resolvido. Como escrever cada configuração está na [referência de configuração](./config).

## Um hook não tem efeito

Depois de `obit init`, um agente conseguiu excluir um arquivo sem ser impedido, ou parece que nada acontece. Há três causas comuns: o agente não foi reiniciado e ainda usa a configuração antiga; a integração desse agente foi instalada em outro escopo (global ou de projeto); ou `OBIT_DISABLED=1` (desliga todos os hooks) ou `OBIT_GUARD_RM=0` (desliga o guardião de exclusão) está definida no ambiente do agente.

Primeiro veja o que está instalado:

```bash
obit init -g --show        # o que está instalado globalmente
obit init --show           # o que está instalado no projeto atual
obit doctor --verbose      # como cada agente está conectado
```

Se não estiver instalado, instale com `obit init -g --agent <nome>`, acrescentando `--dry-run` se quiser ver o plano antes. Reinicie o agente depois; no Codex, execute também `/hooks` e confie nos hooks do Obituary. Para ver o que o hook realmente recebe, defina `OBIT_HOOK_AUDIT=1` no terminal em que você inicia o agente, use o agente por um tempo e então execute `obit hook audit`. Comandos de exclusão negados não precisam desse interruptor, pois são sempre registrados. Os erros do próprio hook vão para `hook.log` no diretório de dados, e a linha `hook-log` do `obit doctor` conta os erros das últimas 24 horas.

Quando a linha do agente no `obit doctor` (por exemplo `claude`) é `PASS` e diz `integration is current`, ele está instalado; se você usa o guardião de exclusão, uma linha como `claude-guard-rm` deve dizer `registered: yes`. Execute `obit doctor` no mesmo terminal em que você inicia o agente: se uma variável de ambiente desligou os hooks, essa linha vira `WARN` e nomeia a variável.

## A proposta de um contêiner Docker não pode ser aprovada

“Approve” na proposta de um contêiner aparece esmaecido na janela, e o cartão diz “Docker is not running: start Docker to decide on this proposal.” Registrar um contêiner significa perguntar ao Docker o ID completo e a hora de criação dele, o que não dá para fazer com o Docker desligado. Inicie o Docker Desktop ou o Docker Engine e aprove de novo; quando for possível clicar em “Approve” e `obit docker list` mostrar o contêiner, funcionou.

Quando um contêiner expira, o Obituary apenas o para e mantém sua camada gravável, e ressuscitá-lo o inicia de novo; um contêiner parado só é excluído depois do período de retenção. Imagens, volumes e redes nunca são tocados.

## Uma atualização falha, ou os downloads a partir da China continental são lentos

`obit update --check` informa um erro, ou o script de instalação está lento ou falha; o mais provável é que este computador não alcance o GitHub. Quando o GitHub demora a responder, `obit update --check` e o script de instalação passam sozinhos para o espelho da China continental. Para usar o espelho desde o começo, execute `bash install.sh --mirror cn` no macOS e no Linux, ou `.\install.ps1 -Mirror cn` no Windows. Se você baixar à mão, pegue o arquivo compactado e o `checksums.txt` da mesma versão e da mesma origem. Quando `obit update --check` imprimir uma linha `Latest:`, está resolvido.
