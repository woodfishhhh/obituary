# Privacy policy

Obituary runs on your own computer. Its ledger, graveyard, logs and settings stay on that computer. Obituary collects no usage data, uploads no crash reports, and never sends your files, their paths or their contents anywhere.

## The one thing the program does on the network: check for a new version

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
