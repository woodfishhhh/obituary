# Quais arquivos são registrados

## O agente e você decidem; o Obituary enterra no prazo

O agente e você decidem o que entra no Obituary (por padrão, `[proposals] mode = "off"`):

- **O agente registra os próprios resultados.** Ele dá uma vida útil com `obit mark <path> 2d` aos scripts de depuração, logs e capturas que cria, usa `obit mktemp 1d` para obter espaço temporário e enterra com `obit bury` o que não sabe se deve guardar. `obit init` informa o agente sobre essa regra.
- **Você adiciona pelo menu de contexto.** No Windows, clique com o botão direito em arquivos ou pastas no Explorador, escolha “Add to Obituary” e selecione 1, 7 ou 30 dias, ou “Bury now (recoverable)”. Veja [Janela, bandeja e menu de contexto](./desktop).
- **Políticas de pastas** atribuem vida útil a pastas como Downloads. Veja [Políticas de pastas](./folder-policies).

Tudo o que for registrado é enterrado quando expira e pode ser ressuscitado como estava durante sete dias. Nada mais entra automaticamente.

- **Arrumação no fim da sessão.** Quando um agente do Claude Code ou Codex está prestes a parar, o Obituary lista os arquivos criados naquela sessão que ainda não foram classificados. Ele pede ao agente que dê vida útil ou enterre scripts, logs e capturas, preservando os entregáveis. O aviso aparece no máximo uma vez por sessão e fica silencioso quando não há nada; outros agentes o recebem no início da sessão seguinte. `[hooks] session_cleanup = false` desativa esse comportamento.
- **Ao atualizar de uma versão antiga**, esta configuração é desligada uma vez. Dependências, saídas de compilação e caches registrados automaticamente antes continuam no lugar, sem vida útil.

## Ativar o registro automático quando você quiser

Ative na página Settings ou Decisions, ou defina `[proposals] mode` em `config.toml`.

- `"tiered"` registra diretamente os arquivos criados pelo agente nesta sessão e os [contêineres Docker](./docker) iniciados para o projeto. O arquivo só entra diretamente se o hook tiver observado a gravação, se não existisse no snapshot inicial e se sua data de criação fosse posterior ao início. O agente é avisado; os itens aparecem em `obit proposals` e em “Handled automatically” na página Decisions. Arquivos anteriores à sessão ou não observados pelo hook viram propostas. `obit approve <id>` registra os itens propostos; depois, as regras correspondentes do projeto podem registrar itens diretamente. `obit reject <id>` recusa o registro automático neste projeto.
- `"auto"` registra diretamente tudo o que o registro automático encontrar e avisa você depois.
- `"ask"` deixa tudo aguardando como proposta.

```powershell
obit proposals           # proposals waiting for you, and what was registered without asking
obit approve 3           # approve: register what it lists
obit reject 3            # reject
obit spare 12            # undo an automatic registration: it stays where it is, without a lifespan
obit extend 12 1w        # keep it longer
```

- **Dependências, saídas de compilação e caches** (`node_modules`, `dist`, `.turbo`, `__pycache__` e semelhantes) nunca são registrados ou propostos automaticamente. Para limpá-los, registre-os você mesmo com `obit mark`.
- **Itens grandes** que não foram criados pelo agente também viram proposta quando passam de 1 GB ou 200 arquivos.
- **Diferença da sessão.** Um snapshot é criado no início e guardado no diretório de dados, sem adicionar nada ao repositório. No fim ele é comparado; correspondências de regras seguem o modo, e arquivos novos sem regra não são registrados nem propostos.
- **Depois de mudar o modo**, o processo em segundo plano reclassifica as propostas pendentes.
- **O Git é consultado outra vez antes do enterro.** Um item registrado por regra ou proposta não é enterrado se já estiver em um commit quando expirar.
- **As propostas expiram sozinhas.** Itens confirmados ou excluídos são removidos; propostas vazias são retiradas, registradas no log de auditoria e não contam como rejeição.

## Regras

```powershell
obit rules                       # the rules in effect here (built-in plus a trusted .obitrules)
obit rules check debug.log out/  # preview what would be registered and by which rule (changes nothing)
obit rules apply debug.log       # register by rule
```

`obit rules apply` registra por regra a qualquer momento; com o registro automático ligado, os hooks também usam essas regras. As regras incluídas cobrem resíduos comuns como `.playwright-mcp/`, `test-results/` e `*.log`. Os backups `*.bak`, `*.bak.*`, `*.orig`, `*.rej`, `*.old`, `*.backup` e `*~` ficam sete dias, mas o `LOG.old` do LevelDB não é tocado. O `.obitrules` na raiz do projeto usa a sintaxe `.gitignore`, com uma vida útil depois de cada padrão.

```
output/**        7d      # every item under output gets its own lifespan
!output/keep/**          # never touch (negation wins, even over registering a parent folder)
test-results/    off     # turn off a built-in rule
```

Diferentemente de `.gitignore`, as negações têm prioridade sobre a última correspondência. No Windows, a comparação ignora maiúsculas e minúsculas. Um padrão inválido, como `[` sem fechar, informa o arquivo e a linha e pausa o registro automático do projeto.

As unidades são `s`, `m`, `h`, `d` e `w`; podem ser combinadas, por exemplo `1d12h`.

## Regras rígidas para registro automático

- Apenas itens não rastreados dentro de uma árvore de trabalho Git são registrados.
- Uma pasta com pelo menos um arquivo rastreado nunca é tocada.
- Se o Git não responder, o item é ignorado.
Uma vida útil existente nunca é encurtada.
- Um projeto que você recusou nunca é registrado diretamente. Políticas de pastas e a confiança em `.obitrules` sempre esperam sua decisão.

## Regras do projeto devem ser confiáveis primeiro

O `.obitrules` do repositório só funciona depois que você o revisa e executa `obit trust`. Assim, um repositório de outra pessoa não pode usar regras para pegar seus arquivos privados. O SHA-256 do arquivo é registrado; qualquer alteração exige uma nova revisão. Sessões não interativas precisam de `--yes`; na CI, defina `OBIT_TRUST_PROJECT_RULES=1`.

## Uma pasta temporária que se enterra

```powershell
$dir = obit mktemp 1d            # .obit/tmp/ in a project (not in Git), otherwise the system temp folder
```

Em bash: `dir=$(obit mktemp 1d)`.
