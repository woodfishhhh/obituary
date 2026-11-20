# Richtlinie zur Codesignatur

[SignPath.io](https://about.signpath.io/) stellt die Codesignatur kostenlos bereit. Das Zertifikat stammt von der [SignPath Foundation](https://signpath.org/).

## Was wird signiert?

- Die Programme `obit.exe`, `obitd.exe` und `obit-gui.exe` in den Release-Archiven.
- Der Release-Workflow in GitHub Actions ([`release.yml`](https://github.com/woodfishhhh/obituary/blob/main/.github/workflows/release.yml)) baut diese Programme automatisch und ausschließlich aus mit einem Tag versehenen Commits dieses Repositorys. Auch die Build-Skripte und die CI-Konfiguration liegen im Repository.
- Jede Signaturanfrage muss eine der unten aufgeführten Personen manuell freigeben.
- Unter Eigenschaften → Details stehen der Produktname „Obituary 讣告“ und die Version.

## Teamrollen

| Rolle | Mitglieder |
| --- | --- |
| Commits und Code-Reviews | [woodfishhhh](https://github.com/woodfishhhh) |
| Freigabe von Signaturanfragen | [woodfishhhh](https://github.com/woodfishhhh) |

Änderungen von Personen außerhalb des Teams prüft ein Maintainer vor dem Zusammenführen. Alle Teammitglieder nutzen auf GitHub die Mehrfaktor-Authentifizierung.

## Datenschutz

Nutzungsstatistiken und die automatische Suche nach Updates lassen sich unabhängig voneinander abschalten. Einzelheiten stehen in der [Datenschutzerklärung](privacy).
