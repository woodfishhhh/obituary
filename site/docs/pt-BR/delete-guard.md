# Guardião de exclusão

Excluir uma pasta pessoal ou uma unidade inteira já não é uma hipótese. Com o guardião ligado, se o agente executar `rm -rf build`, `rmdir`, `git clean -fdx`, `Remove-Item -Recurse` ou uma exclusão semelhante, o Obituary interrompe a execução e mostra o comando equivalente, como `obit rm -rf build`. Depois que o agente o usa, os itens vão para o cemitério em vez de desaparecer e `obit resurrect <id>` os restaura.

```powershell
obit init -g --guard-rm                 # Claude Code; add --agent codex for Codex. Off by default
```

A proteção contra exclusão reconhece `docker volume rm`, `docker volume remove`, `docker volume prune`, `docker compose down -v` (ou `--volumes`) e `docker system prune --volumes`. Ela só sugere `obit bury --docker-volume` para um `docker volume rm` simples com nomes explícitos.

## Uma execução real

Com o guardião ativo, o hook recebeu uma chamada Bash do Claude Code, `rm -rf build`, e respondeu assim; `rm` nunca foi executado:

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Obituary's delete guard is on: `'C:/work/obit.exe' rm -rf build` provides restorable deletion; `obit resurrect` restores graveyard entries."}}
```

O agente muda para o Obituary; a pasta vai para o cemitério e um comando a traz de volta.

```console
$ obit rm -rf build
$ obit graveyard --here
ID  BURIED            PURGE  SIZE  PATH
#2  2026-11-11 00:36  in 7d  23 B  C:\work\build
$ obit resurrect --last
#2 resurrected  C:\work\build
```

## Princípios

- **Recuse, não reescreva.** O Obituary não altera o comando do agente. O Claude Code verifica as permissões contra o comando reescrito, o que poderia invalidar uma regra `deny` para o original. Recusar o original e deixar o agente emitir o equivalente preserva todas as regras de permissão.
- **Toda exclusão literal é interrompida.** `rm`, `rmdir`, `git clean` ou `Remove-Item` (incluindo `del`, `rd`, `ri`) em qualquer posição, depois de `&&`, `||` ou `;`, em um pipeline, `$(…)`, subshell, condição ou loop, interrompe a execução. Isso inclui `rm -rf build && npm run build`, `rm -rf build 2>/dev/null` e `git clean -fdx && npm ci`. `rm` em strings, comentários ou argumentos de `echo` não conta, nem as prévias `rm --help`, `git clean -n` e `Remove-Item -WhatIf`.
- **Liste antes os alvos escolhidos em tempo de execução.** `find -delete`, `find -exec rm` (incluindo `-execdir`), `xargs rm` e `Get-ChildItem … | Remove-Item` do PowerShell são recusados sem reescrita. Revise as correspondências e só então passe os caminhos explícitos a `obit rm -- <path>...`. Um `find` que apenas pesquisa ou lista caminhos continua funcionando. O guardião não inspeciona wrappers arbitrários como `sudo rm` ou `bash -c '…'`.
- **Nunca cai em uma exclusão real.** Todo caminho entregue a `obit rm`, `obit remove-item` ou `obit git-clean` é enterrado. Quando o caminho é recusado, o erro segue o formato de `rm` e o código de saída é diferente de zero.
- **O caminho real é verificado.** Entre a intenção do agente e o comando executado existem aspas, escapes e às vezes um proxy. Se um caractere desaparecer, `rm -rf "$OUT"/` pode virar `rm -rf /`. Antes de agir, `obit rm` confere o caminho real no disco.
- **No Windows, o Codex verifica substituições dentro de aspas.** Outra shell pode executar texto que o Bash manteria literal. Cada `$(…)` é lido pelas regras do Bash e do PowerShell, inclusive em aspas, heredocs citados e comentários. Uma exclusão interrompe a execução; texto com backticks pode ser escrito em arquivo e passado como caminho.
- Arquivos rastreados pelo Git também são enterrados para que alterações não commitadas possam ser recuperadas. É uma exclusão deliberada do agente, diferente do registro automático.
- `obit rm -rf` em uma pasta com `.git` enterra o repositório inteiro. Um `rm -rf` real o destruiria, enquanto o enterro pode ser desfeito. Proteções de pastas do sistema e do usuário continuam valendo.
- **O comando equivalente é deliberadamente restrito.** Ele nunca pode selecionar mais do que o original. Por isso o Obituary não traduz entre shells e só oferece equivalentes para os formatos da tabela. Nos demais casos, a exclusão é interrompida e são indicados `obit bury <path>` e `obit git-clean` como exclusões recuperáveis.

## O que é parado e o que passa

| Host | Interrompido com comando equivalente | Interrompido com motivo e orientação | Passa |
|---|---|---|---|
| Claude Code (ferramenta Bash) | `rm`, `rmdir`, `git clean`, opcionalmente precedidos por `cd x` literal ligado por `&&`, `;` ou nova linha; `rm -f *.log` quando nenhum arquivo começa com `-`, `rm -rf ../dist`, `rm -- "$target"` com a variável após `--` | Qualquer outra exclusão literal: outras instruções (`npm …`, `trap`, `hash`), variáveis antes de `--`, caminhos POSIX absolutos iniciados por `/`, redirecionamentos, `\|\|`, pipelines, `$(…)`, exclusões interativas, `/usr/bin/rm` com caminho | Comandos que não excluem |
| Codex, no `/mnt/<drive>` do WSL | Um único `rm` ou `rmdir` com argumentos de caminhos relativos sem `..`, `~`, curingas ou aspas | Qualquer outra exclusão literal, incluindo `git clean` e caminhos para o sistema de arquivos próprio do WSL | Diretório de trabalho no sistema de arquivos próprio do WSL |
| Codex declarado como Windows PowerShell | Um único `Remove-Item` ou alias com um caminho literal (`-Recurse` e `-Force` permitidos) | Qualquer outra exclusão literal: várias instruções, pipelines, parâmetros como `-ErrorAction`, caminhos de provider, `~`, curingas ou backticks | Texto entre aspas curvas |
| Codex no Windows com shell desconhecido | — | Exclusão dentro de fragmento raw de backtick ou `$(…)` | Outros comandos |
| pwsh no Linux, `git rm` | — | — | Tudo passa |

Para fornecer um equivalente, a exclusão deve ser o **último segmento** do comando; os segmentos anteriores só podem ser ligados por `&&`, `;` ou novas linhas. Não há equivalente para nomes terminados em ponto ou espaço, que o Windows normaliza. A exclusão literal continua sendo bloqueada.

## Use-o você mesmo

`obit rm`, `obit rmdir` e `obit git-clean` aceitam as opções não interativas comuns de `rm`, `rmdir` e `git clean`. Opções sem suporte como `-i` ou `rmdir -p` causam erro. `obit remove-item` é uma versão simples de `Remove-Item` e aceita apenas um caminho literal; para curingas, use `obit rm *.log` no Bash.

Desative temporariamente com `OBIT_GUARD_RM=0`. `obit stats` mostra quantas exclusões foram interrompidas e quanto foi enterrado.
