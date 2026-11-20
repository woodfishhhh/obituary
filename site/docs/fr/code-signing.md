# Politique de signature du code

[SignPath.io](https://about.signpath.io/) fournit gratuitement le service de signature de code. Le certificat est fourni par la [SignPath Foundation](https://signpath.org/).

## Ce qui est signé

- Les exécutables `obit.exe`, `obitd.exe` et `obit-gui.exe` fournis dans les archives de distribution.
- Le workflow de publication de GitHub Actions ([`release.yml`](https://github.com/woodfishhhh/obituary/blob/main/.github/workflows/release.yml)) compile automatiquement ces programmes, uniquement à partir des commits étiquetés de ce dépôt. Les scripts de compilation et la configuration de la CI se trouvent dans le même dépôt.
- Chaque demande de signature est approuvée manuellement par une personne habilitée, indiquée ci-dessous.
- Les propriétés du fichier, sous Détails, indiquent le nom du produit « Obituary 讣告 » et sa version.

## Rôles de l'équipe

| Rôle | Membres |
| --- | --- |
| Personnes qui intègrent et relisent les modifications | [woodfishhhh](https://github.com/woodfishhhh) |
| Personnes habilitées à approuver les signatures | [woodfishhhh](https://github.com/woodfishhhh) |

Un responsable du projet vérifie toute modification proposée par une personne extérieure à l’équipe avant de la fusionner. Tous les membres utilisent l’authentification multifacteur sur GitHub.

## Confidentialité

Les statistiques d’utilisation et la recherche automatique de mises à jour peuvent être désactivées séparément. Consultez la [politique de confidentialité](privacy).
