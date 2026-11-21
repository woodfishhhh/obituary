# Solução de problemas

Comece com `obit doctor`. Ele verifica instalação, integrações, serviços em segundo plano e dados sem alterar arquivos. Use `--json` ou `--verbose` para obter mais detalhes.

## PATH e hashes

**Sintoma:** o shell não encontra `obit` ou o hash do arquivo não confere.

**Solução:** baixe `checksums.txt` da mesma versão e execute `sha256sum --check --strict checksums.txt`. No PowerShell, compare `Get-FileHash .\obit-*.zip -Algorithm SHA256`. Não execute um arquivo diferente e abra outro terminal após alterar o PATH.

## Desktop

No Windows, verifique SHA-256 antes de escolher “Mais informações → Executar assim mesmo” no SmartScreen. Se faltar WebView2, instale Microsoft Edge WebView2 Runtime. A janela do Windows oferece apenas x64 por enquanto.

O app do macOS não tem assinatura Developer ID nem notarização. Verifique o hash e permita a primeira abertura em “Ajustes do Sistema → Privacidade e Segurança → Abrir assim mesmo”. Não desative o Gatekeeper nem permita um app marcado como danificado ou malicioso. Para o LaunchAgent, confira `obit daemon status` e `launchctl list | grep obituary`.

A janela do Ubuntu 24.04 precisa de GTK 3 e WebKitGTK 4.1:

```bash
sudo apt update
sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
obit doctor
```

Confira os agendadores com `systemctl --user status obituary.timer`, `systemctl --user list-timers | grep obituary` e `crontab -l`.

## Segundo plano, atualizações e arquivos

Use `obit daemon status`, `obit sweep` para uma execução imediata e `obit daemon enable` para restaurar o agendamento. O espelho chinês usa `bash install.sh --mirror cn`. Não misture arquivos de versões diferentes.

Depois de renomear uma unidade montada pelo WSL, confirme o caminho real do Windows. Arquivos bloqueados permanecem no lugar e são tentados novamente; erros de TOML aparecem com arquivo e linha em `obit config`.

## Integrações e contêineres

Se um hook não funcionar, execute `obit init --show`, `obit init --dry-run` e `obit doctor --verbose`, verificando que `OBIT_DISABLED=1` não está definido. Use `OBIT_HOOK_AUDIT=1 obit hook audit` para auditar. Inicie o Docker antes de aprovar um contêiner. Uma restauração nunca sobrescreve o destino existente: usa um nome sem conflito.
