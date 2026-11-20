# Avisos do cemitério

Enterrar um arquivo em vez de excluí-lo só ajuda se for possível encontrá-lo depois. Quando uma ferramenta do agente acessa um caminho enterrado que ainda pode ser restaurado, ou algum item dentro de um diretório enterrado, o Obituary informa:

- o número do túmulo;
- quando o item foi enterrado;
- quanto tempo resta para ressuscitá-lo;
- `obit resurrect <id>`.

## O aviso não interfere na execução

- A chamada de ferramenta nunca é bloqueada ou alterada.
- Cada túmulo é mencionado uma vez por sessão.
- Claude Code e Codex recebem o aviso antes da execução da ferramenta. Para os demais agentes, `obit init --show` informa em que momento o aviso aparece.
- Fora disso, não há interrupções. Ao iniciar o trabalho, o agente só precisa saber que o Obituary está disponível, sem ler uma lista antes.

Se você já instalou o Obituary, execute `obit init` novamente para ativar os avisos do cemitério.

## Resumo da sessão (opcional)

Para economizar tokens, no início da sessão o agente só recebe as propostas que precisam da sua decisão e os itens que acabaram de ser registrados automaticamente. Para incluir o resumo, ative “Session summary” nas configurações da janela ou use esta configuração:

```toml
[hooks]
session_summary = true
```

O agente também verá os itens do projeto que expiram nas próximas 24 horas e os que foram enterrados recentemente e ainda podem ser ressuscitados. Os diretórios temporários de `obit mktemp` ficam de fora.

## Consulte o cemitério

```powershell
obit graveyard           # what is in the graveyard and when it will be purged
obit graveyard --here    # this folder only
obit resurrect <id>      # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last    # undo the last deletion: everything one obit bury / obit rm buried comes back
```
