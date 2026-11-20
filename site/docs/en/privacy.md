# Privacy policy

Obituary's ledger, graveyard, logs and settings stay on your computer. Obituary never uploads crash reports, files, paths or file contents.

## Usage statistics

Statistics are on by default and are used only to count active installations and measure version distribution. During actual use, Obituary may send a random installation ID, version, operating system, architecture, interface language and installation channel at most once every 24 hours. It does not send files, paths or commands, or store IP addresses or User-Agent headers. Run `obit config telemetry off` or set `OBIT_TELEMETRY=0` to disable reporting. Nothing is reported in CI or when `DO_NOT_TRACK=1` is set.

The server stores neither IP addresses nor user agents nor raw installation IDs. A separately keyed digest can link reports from the same installation, not identify a person. Daily activity expires after 400 days; only the digest and first and last observation dates are kept long term for deduplication. Successful reports per channel are counted in aggregate. Only the maintainers can view the authenticated dashboard.

Disabling reporting clears the local installation ID and any pending report. Update checks have a separate switch.

## Checking for a new version

- At most once a day it reads public release information: the latest release of the Obituary repository on `https://api.github.com`, or, when GitHub does not answer, `https://cn.obituary.woodfish.site/download/latest.json` on the mirror in mainland China.
- The request carries only what any web request carries: your IP address and a User-Agent with Obituary's version (for example `Obituary/1.0.0`).
- Turn it off at any time: switch off "Check for updates automatically" in the window's Settings, or put this in `%APPDATA%\obituary\config.toml`:

  ```toml
  [update]
  check = false
  ```

  Once it is off, Obituary only checks when you run `obit update --check` yourself.

## Installing and downloading

- Scoop, winget and the install script download the archives from [GitHub Releases](https://github.com/woodfishhhh/obituary/releases); [GitHub's privacy statement](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement) applies.
- With the mirror (`-Mirror cn`) they come from `cn.obituary.woodfish.site`, whose server keeps ordinary access logs (time, IP address, requested URL) for troubleshooting.

## Connecting agents

`obit init` changes only the agents' configuration files on your computer, showing you the plan first. It sends nothing to the agents' vendors or anywhere else.

## Website

- The website <https://obituary.woodfish.site/> uses no analytics or tracking scripts. Visitors in mainland China are served by our server and everyone else by GitHub Pages; both keep ordinary access logs.
- An email address left on the website is used only to announce new platforms and versions. It is stored on our own server, never shared with third parties, and every email has an unsubscribe link.

## Contact

Ask privacy questions in a [GitHub issue](https://github.com/woodfishhhh/obituary/issues); report security problems privately as [SECURITY.md](https://github.com/woodfishhhh/obituary/blob/main/SECURITY.md) describes.
