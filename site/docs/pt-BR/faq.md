# Perguntas frequentes

## Por que não excluir diretamente?

Se você excluir de vez um arquivo e perceber depois que precisava dele, não poderá recuperá-lo. O Obituary o enterra no cemitério e o mantém por sete dias por padrão. Durante esse prazo, `obit resurrect` o devolve ao lugar de origem; só depois ele é excluído permanentemente. Se o agente tentar usá-lo nesses sete dias, recebe na hora o número do túmulo e o comando de ressurreição (veja [Avisos do cemitério](./grave-notices)).

## Funciona no macOS e no Linux?

Sim. Ambos oferecem a linha de comando e programas em segundo plano para x64 e ARM64, com cemitério, ressurreição e enterro automático. Para verificar os vencimentos, o macOS usa o launchd e o Linux, um temporizador de usuário do systemd. Os hooks do agente também iniciam verificações quando necessário. Veja o [guia de início rápido](./index). No Windows e no Linux, `graveyard.mode = "recycle"` usa a Lixeira do sistema. Restaure os arquivos pela Lixeira, não com `obit resurrect`. O macOS só oferece o cemitério.

## E se aparecer “O Windows protegeu o computador”?

O Obituary ainda não tem assinatura de código. Se você baixou o arquivo compactado pelo navegador e apareceu o aviso, escolha “Mais informações → Executar assim mesmo”. O Scoop, o winget e o script de instalação geralmente não acionam esse aviso. Os três conferem primeiro o SHA-256 do arquivo.

## A janela está disponível em ARM64?

Não. `obit-gui.exe` só está disponível em x64. A versão ARM64 inclui a linha de comando e a bandeja do sistema.

## Meu código pode ser registrado antes de entrar em um commit?

Não. O registro automático vem desligado por padrão: só entram os itens que o agente registra com `obit mark` e os que você adiciona pelo menu de contexto. Mesmo quando está ligado, só considera itens não rastreados em uma árvore de trabalho do Git. Arquivos novos que não correspondem a nenhuma regra, como código ainda não incluído em um commit, nunca aparecem na lista. Dependências, saídas de compilação e caches são preservados. Um item registrado por regra ou proposta não é enterrado se já estiver em um commit quando expirar. Veja [O que é registrado](./auto-register).

## E se o Git falhar ou o meu `.gitignore` for abrangente demais?

Antes de registrar qualquer item, o Obituary pergunta ao Git se ele é rastreado. Se o Git não puder responder, por não estar instalado, por tempo limite ou por danos no repositório, nada será registrado. Pastas fora de um repositório Git nunca são registradas automaticamente: só você ou o agente podem atribuir vida útil a elas com `obit mark`. Um `.gitignore` amplo apenas deixa mais itens sem rastreamento. O registro automático continua limitado às regras internas e ao arquivo `.obitrules` que você marcou como confiável.

## Um agente pode tratar meus arquivos como se fossem dele para pular a proposta?

Não. Por padrão o registro automático vem desligado e nada é registrado automaticamente sem sua aprovação. No modo em camadas, só os arquivos comprovadamente criados pelo agente naquela sessão entram sem proposta: o hook observou a gravação, o arquivo não aparece no retrato inicial da sessão e o horário de criação no sistema de arquivos é posterior ao início dela. Arquivos anteriores à sessão ou criados por outro programa continuam sujeitos a proposta. Mesmo que o agente seja induzido a renomear uma pasta antiga para corresponder a uma regra, a data de criação não muda: haverá apenas uma proposta. Arquivos rastreados pelo Git nunca são registrados automaticamente, em nenhum modo. Você é avisado de cada registro direto e pode desfazê-lo com `obit spare`. O que foi enterrado pode ser ressuscitado por sete dias.

## Um agente pode aprovar propostas para mim?

Não. Não há ferramentas MCP para aprovar ou rejeitar propostas, confiar em `.obitrules`, excluir permanentemente itens do cemitério ou alterar configurações, inclusive o modo de propostas. O agente só pode encaminhar a proposta para você.

## `obit resurrect` funciona no modo de lixeira?

Não. Com `mode = "recycle"` em `config.toml`, os itens enterrados vão para a Lixeira do sistema e precisam ser restaurados de lá.

## Como desativar temporariamente?

- Hooks: `OBIT_DISABLED=1`
- Guardião de exclusão: `OBIT_GUARD_RM=0`
- Limpeza automática: selecione “Pause” no menu da bandeja.

## Onde estão os dados?

`%LOCALAPPDATA%\obituary` contém o registro, o log de auditoria, o log do daemon e o cemitério desta unidade. A configuração fica em `%APPDATA%\obituary\config.toml`. Veja [Como funciona](./how-it-works#where-the-data-lives).

## A desinstalação apaga o cemitério?

Não. `obit uninstall` mantém o cemitério e o registro por padrão. Só acrescente `--purge-data` se quiser excluir os dois. Veja [Desinstalação](./uninstall).
