# Política de assinatura de código

A [SignPath.io](https://about.signpath.io/) fornece o serviço de assinatura de código gratuitamente. O certificado é fornecido pela [SignPath Foundation](https://signpath.org/).

## O que é assinado

- Os executáveis `obit.exe`, `obitd.exe` e `obit-gui.exe` incluídos nos arquivos compactados de cada versão.
- O fluxo de publicação no GitHub Actions ([`release.yml`](https://github.com/woodfishhhh/obituary/blob/main/.github/workflows/release.yml)) compila esses programas automaticamente, usando apenas commits deste repositório que tenham uma tag. Os scripts de compilação e a configuração de CI ficam no mesmo repositório.
- Cada solicitação de assinatura precisa da aprovação manual de uma das pessoas listadas abaixo.
- Em Propriedades → Detalhes, os programas exibem o nome do produto “Obituary 讣告” e a versão.

## Funções da equipe

| Função | Membros |
| --- | --- |
| Responsáveis por integrar e revisar alterações | [woodfishhhh](https://github.com/woodfishhhh) |
| Responsáveis por aprovar assinaturas | [woodfishhhh](https://github.com/woodfishhhh) |

Um mantenedor revisa cada alteração enviada por alguém de fora da equipe antes de integrá-la. Todos os membros usam autenticação multifator no GitHub.

## Privacidade

É possível desativar separadamente as estatísticas de uso e a verificação automática de atualizações. Consulte a [política de privacidade](privacy).
