# Política de privacidade

O registro, o cemitério, os logs e as configurações do Obituary ficam no seu computador. O Obituary não envia relatórios de falhas, arquivos, caminhos nem conteúdo de arquivos.

## Estatísticas de uso

As estatísticas vêm ativadas e servem apenas para contar as instalações ativas e medir a distribuição das versões. Durante o uso efetivo do programa, o Obituary pode enviar, no máximo uma vez a cada 24 horas, um identificador de instalação aleatório, a versão, o sistema operacional, a arquitetura, o idioma da interface e o canal de instalação. Não envia arquivos, caminhos nem comandos, e não armazena endereços IP nem cabeçalhos User-Agent. Execute `obit config telemetry off` ou defina `OBIT_TELEMETRY=0` para desativar o envio. Nada é enviado em ambientes de integração contínua (CI) ou quando `DO_NOT_TRACK=1` está definido.

O servidor não armazena endereços IP, agentes de usuário nem identificadores de instalação originais. Um resumo calculado com uma chave separada permite associar relatórios da mesma instalação, mas não identificar uma pessoa. Os dados de atividade diária expiram após 400 dias. Apenas o resumo e as datas da primeira e da última observação são mantidos a longo prazo para evitar duplicatas. Os relatórios enviados com sucesso são contabilizados apenas como totais por canal. Somente a equipe de manutenção pode acessar o painel mediante autenticação.

Desativar o envio apaga o identificador de instalação local e qualquer relatório pendente. A verificação de atualizações tem um controle separado.

## Verificação de novas versões

- No máximo uma vez por dia, o Obituary consulta os dados públicos da versão mais recente do repositório em `https://api.github.com`. Se o GitHub não responder, consulta `https://cn.obituary.woodfish.site/download/latest.json` no espelho da China continental.
- A solicitação contém apenas os dados habituais de uma requisição web: seu endereço IP e um User-Agent com a versão do Obituary, por exemplo `Obituary/1.0.0`.
- Para desativar a verificação a qualquer momento, desligue “Check for updates automatically” nas configurações da janela ou inclua o seguinte em `%APPDATA%\obituary\config.toml`:

  ```toml
  [update]
  check = false
  ```

  Depois disso, o Obituary só verifica atualizações quando você executa `obit update --check`.

## Instalação e downloads

- O Scoop, o winget e o script de instalação baixam os arquivos compactados do [GitHub Releases](https://github.com/woodfishhhh/obituary/releases). Aplica-se a [declaração de privacidade do GitHub](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement).
- Com o espelho (`-Mirror cn`), os arquivos vêm de `cn.obituary.woodfish.site`. O servidor mantém logs de acesso comuns (horário, IP e URL solicitada) para diagnosticar problemas.
- As atualizações pelo aplicativo («Update now» ou «Download and restart to update» na janela, e `obit update --apply`) baixam o arquivo da versão e o `checksums.txt` do GitHub Releases, ou de `cn.obituary.woodfish.site` quando o GitHub não responde em dez segundos. As requisições levam apenas o endereço IP e um User-Agent com a versão do Obituary, e nenhum dado de uso é enviado.
- A cada lançamento, o GitHub Actions envia os arquivos ao espelho. O espelho verifica e distribui os arquivos localmente, sem encaminhar sua solicitação de download ao GitHub.

## Conexão de agentes

`obit init` modifica apenas os arquivos de configuração dos agentes no seu computador e mostra o plano antes de fazer alterações. Não envia nada aos fornecedores dos agentes nem a qualquer outro destino.

## Site

- O site <https://obituary.woodfish.site/> não usa scripts de análise de tráfego ou rastreamento. Nosso servidor atende os visitantes da China continental, e o GitHub Pages atende os demais. Ambos mantêm logs de acesso comuns.
- O endereço de e-mail informado no site só é usado para anunciar novas plataformas e versões. Ele fica no nosso próprio servidor e não é compartilhado com terceiros. Todo e-mail inclui um link para cancelar a inscrição.

## Contato

Para dúvidas sobre privacidade, abra uma [issue no GitHub](https://github.com/woodfishhhh/obituary/issues). Relate problemas de segurança em particular conforme as instruções de [SECURITY.md](https://github.com/woodfishhhh/obituary/blob/main/SECURITY.md).
