# Políticas de pastas

Uma política de pasta atribui uma vida útil a cada item que chega à pasta. Um exemplo comum é a pasta Downloads:

```powershell
obit watch C:\Users\you\Downloads 2w        # files that land in Downloads are buried two weeks later
obit watch --list                           # folder policies, their exemptions and quiet periods
obit watch --remove C:\Users\you\Downloads  # remove a policy and the lifespans it registered
```

No macOS e no Linux, depois da aprovação, cada item diretamente na pasta tem seu próprio prazo, contado por padrão desde a primeira detecção. A pasta monitorada permanece protegida; subpastas vencidas só geram propostas, e arquivos temporários de download são ignorados. Se não for possível impedir gravações, o item permanece no lugar mesmo após dez minutos sem alterações.

**No macOS e no Linux, a política ainda não pode enterrar arquivos vencidos nem mover uma subpasta após a aprovação.**

```sh
obit watch "$HOME/Downloads" 2w
```

## Como a política funciona

- **A política precisa da sua aprovação.** `obit watch` cria uma proposta. Ela entra em vigor quando você executa `obit approve` ou aprova a proposta na página “Para você” da janela.
- **O arquivo precisa ficar dez minutos sem alterações.** Ele só é enterrado se a vida útil tiver expirado e não houver alterações nos últimos dez minutos.
- **Cada subpasta tem uma proposta separada.** Quando a vida útil da subpasta expira, o Obituary apenas cria uma proposta. Ela só é enterrada depois da sua aprovação.
- Remover uma política cancela as vidas úteis que ela registrou.

## Datas no Explorador de Arquivos {#dates-in-explorer}

Em Downloads, na Área de Trabalho e nas pastas com uma política, o Explorador exibe os itens monitorados como “name (11-13 burial)”. O nome real do arquivo não muda. O rótulo segue o idioma da interface do Obituary: em chinês simplificado, aparece como “name（11-13 下葬）”; nos demais casos, fica em inglês.

- As datas vêm do arquivo oculto `desktop.ini` da pasta.
- Na seção `[explorer]` de `config.toml`, use `enabled = false` para ocultar as datas ou `everywhere = true` para exibi-las em todos os locais.
- O Explorador mantém as visualizações de pasta em cache. Depois de prolongar a vida útil, a mesma janela pode mostrar a data anterior por até um minuto.
