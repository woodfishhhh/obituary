# Dépannage

Commencez par `obit doctor`. Il vérifie l’installation, les intégrations, les services en arrière-plan et les données sans modifier les fichiers. Ajoutez `--json` ou `--verbose` pour plus de détails.

## PATH et sommes de contrôle

**Symptôme :** le shell ne trouve pas `obit` ou l’archive ne correspond pas à sa somme.

**Solution :** téléchargez `checksums.txt` de la même version et exécutez `sha256sum --check --strict checksums.txt`. Dans PowerShell, comparez `Get-FileHash .\obit-*.zip -Algorithm SHA256`. Ne lancez pas une archive différente et rouvrez le terminal après une modification du PATH.

## Ordinateurs de bureau

Sous Windows, vérifiez SHA-256 avant de choisir « Plus d’informations → Exécuter quand même » dans SmartScreen. Si WebView2 manque, installez Microsoft Edge WebView2 Runtime. La fenêtre Windows est actuellement limitée à x64.

L’app macOS n’est ni signée Developer ID ni notariée. Vérifiez la somme, puis autorisez la première ouverture dans « Réglages système → Confidentialité et sécurité → Ouvrir quand même ». Ne désactivez pas Gatekeeper et n’autorisez pas une app déclarée endommagée ou malveillante. Pour LaunchAgent, utilisez `obit daemon status` et `launchctl list | grep obituary`.

Ubuntu 24.04 nécessite GTK 3 et WebKitGTK 4.1 :

```bash
sudo apt update
sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
obit doctor
```

Vérifiez les planifications avec `systemctl --user status obituary.timer`, `systemctl --user list-timers | grep obituary` et `crontab -l`.

## Arrière-plan, mises à jour et fichiers

Consultez `obit daemon status`, lancez `obit sweep` pour un passage immédiat et `obit daemon enable` pour réactiver la planification. Le miroir chinois utilise `bash install.sh --mirror cn`. Ne mélangez pas archive et somme de contrôle de versions différentes.

Après un changement de nom d’un disque WSL, confirmez le vrai chemin Windows. Un fichier verrouillé reste en place et sera réessayé ; corrigez le TOML indiqué par `obit config`.

## Intégrations et conteneurs

Pour un hook inactif, exécutez `obit init --show`, `obit init --dry-run` et `obit doctor --verbose`, puis vérifiez l’absence de `OBIT_DISABLED=1`. L’audit est disponible avec `OBIT_HOOK_AUDIT=1 obit hook audit`. Démarrez Docker avant d’approuver un conteneur. Une restauration ne remplace jamais une cible existante : Obituary choisit un nom sans conflit.
