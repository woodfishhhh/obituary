# Contêineres e volumes do Docker

Os contêineres iniciados em uma sessão do agente também podem ter uma vida útil.

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

## Definir a vida útil ao iniciar um contêiner

Contêineres temporários ficam esquecidos com mais facilidade quando ninguém define o prazo ao criá-los. Como `docker run -d` imprime o ID completo na saída padrão, você pode passá-lo imediatamente para `obit docker add`:

```powershell
# PowerShell
$cid = docker run -d --name tmp-redis redis:7
if ($LASTEXITCODE -eq 0) { obit docker add $cid 4h }
```

```bash
# Bash / zsh (no name required; pass the returned ID directly)
cid=$(docker run -d redis:7) && obit docker add "$cid" 4h
```

`obit docker add <name-or-id> <lifespan>` aceita o nome do contêiner, um ID curto ou o ID completo de 64 caracteres, e não tem as opções `--at` nem `--note`. Ao registrar, imprime o número no registro e o horário de expiração (`#<id> docker:<name> (<12-char-id>)  expires <time>`). Não passe `--rm` para `docker run`: o Docker remove contêineres com AutoRemove assim que param, por isso `obit docker add` recusa registrá-los. Antes de expirar, use o número listado em `obit docker list` ou `obit list` com `obit extend <id> <lifespan>` ou `obit spare <id>`.

## Consultar e ressuscitar contêineres parados durante a retenção

Quando um contêiner expira e o `docker stop` é executado, ele sai de `obit docker list` (que mostra apenas contêineres ativos em acompanhamento) e vai para o cemitério por `[graveyard] retention_days` (7 dias por padrão). Use estes comandos para revisar ou reativar contêineres parados durante o período de retenção:

```powershell
obit graveyard                 # list all restorable burials; containers show as docker:<name> (<12-char-id>) with purge countdown
obit graveyard --json          # machine-readable output; filter burials where kind == "container"
obit stats                     # total graveyard count and the next purge time
obit resurrect <id>            # start the stopped container by its numeric graveyard ID and stop tracking it
```

Contêineres registrados manualmente com `obit docker add` não ficam vinculados a um diretório de projeto, portanto não aparecem em `obit graveyard --here`; apenas os registrados automaticamente ou aprovados a partir de uma proposta de projeto correspondem a `--here`. No desktop, o menu da bandeja mostra o total no cemitério e até os 10 enterros mais recentes (clique em um para ressuscitar), e a página Cemitério da janela permite filtrar por nome, ver o tempo restante e ressuscitar os itens selecionados. O serviço em segundo plano não envia notificações proativas do sistema quando um contêiner para ou se aproxima da exclusão definitiva; confira `obit graveyard` ou a página Cemitério sempre que quiser revisar os contêineres parados antes do fim da retenção.

## Definir um prazo para um volume ou enterrá-lo agora

Volumes nunca são registrados automaticamente. O Obituary só trata os que você registra ou enterra explicitamente e não exclui imagens nem redes.

Use `obit mark --docker-volume cache 3d` (ou `add`) para enterrar `cache` em três dias, ou `obit bury --docker-volume -- cache` para enterrá-lo agora.

O Docker precisa estar no modo de contêineres Linux. Só são aceitos volumes locais com o driver `local` e sem opções extras. No primeiro uso, se a imagem auxiliar ainda não estiver disponível, o Docker baixa uma versão fixa do Debian.

Reserve espaço no cemitério para um arquivo completo. O volume só é excluído depois de arquivado e verificado. Se algum contêiner fizer referência a ele, mesmo parado, ou se o caminho se sobrepuser a uma montagem bind, o volume permanece intacto.

`obit resurrect <id>` recria o volume com o nome original e o rótulo permanente `io.obituary.restore`. Se o nome já estiver em uso, o volume existente não será sobrescrito, mesmo que esteja vazio.

Os eventos do Docker são verificados antes e depois da exclusão. Se não for possível confirmá-la, mantenha o arquivo verificado e consulte `obit doctor`; não considere o volume enterrado. Cada aviso de `docker-volume-recovery` identifica uma entrada a verificar. Siga as instruções para consultar o volume no daemon Docker original. Nem `obit resurrect` nem `obit purge` aceitam arquivos ainda não registrados.

A proteção contra exclusão reconhece `docker volume rm`, `docker volume remove`, `docker volume prune`, `docker compose down -v` (ou `--volumes`) e `docker system prune --volumes`. Ela só sugere `obit bury --docker-volume` para um `docker volume rm` simples com nomes explícitos.

## Ao expirar, o contêiner para

- Depois de `docker stop`, o contêiner e sua camada gravável continuam intactos. `obit resurrect <id>` executa `docker start` e encerra o acompanhamento da vida útil.
- O contêiner só é removido com `docker rm` após o período de retenção (`[graveyard] retention_days`, 7 dias por padrão). As opções `-v` e `-f` nunca são usadas.
- Se você iniciar o contêiner durante o período de retenção, o Obituary o mantém e deixa de acompanhá-lo, mesmo que você o pare novamente. As versões anteriores não registravam a hora de início: esses contêineres não são removidos e ainda podem ser recuperados com `obit resurrect <id>`.
- `obit purge` e `--all` também respeitam o período de retenção dos contêineres. O modo de lixeira não se aplica: eles são parados primeiro e removidos só após o fim do prazo.

## Identificação pelo ID completo

- O nome é usado apenas para exibição. Antes de parar, iniciar ou remover um contêiner, o Obituary compara o ID completo e o horário de criação com os dados registrados. Se houver divergência, não faz nada.
- O Obituary registra o contexto do Docker e o daemon ao qual o contêiner pertence. Ele só atua nesse daemon; trocar de `docker context` não redireciona a operação.
- Se você remover um contêiner fora do Obituary, a remoção externa é registrada e a entrada de acompanhamento é retirada.
- O Docker exclui os contêineres iniciados com `--rm` (AutoRemove) assim que eles param. Por isso, o Obituary se recusa a registrá-los.

## Registro automático dos contêineres de uma sessão

Por padrão, o registro automático vem desligado. O Obituary não procura contêineres, mas você pode registrar um com `obit docker add`. Se ativar o recurso, ao fim da sessão o Obituary procura em segundo plano os contêineres criados durante ela. Só considera os que têm o diretório de trabalho do Compose ou uma montagem do tipo bind dentro do projeto. Os demais podem ter sido iniciados em outro lugar e não são alterados. Os encontrados recebem três dias de vida útil, e você é avisado depois. Se cancelar com `obit spare <id>`, a mesma sessão não registrará o contêiner de novo. Se a sessão foi iniciada fora de um projeto Git, por exemplo na sua pasta pessoal, ou se várias sessões reivindicam o mesmo contêiner, não dá para determinar quem o iniciou: fica uma proposta à espera da sua aprovação. Com `[proposals] mode = "ask"`, o Obituary também cria uma proposta e só acompanha o contêiner após `obit approve`.

Cada contêiner gera no máximo uma proposta. Os hooks nunca chamam o Docker. Se ele não estiver instalado ou em execução, essa etapa é ignorada sem aviso; `obit doctor` explica o motivo. Não é possível aprovar uma proposta de contêiner com o Docker parado. `obit proposals` e a página de decisões da janela informam essa condição. A aprovação fica disponível assim que o Docker inicia.
