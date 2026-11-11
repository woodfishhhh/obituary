# Code signing policy

Free code signing provided by [SignPath.io](https://about.signpath.io/), certificate by [SignPath Foundation](https://signpath.org/).

## What is signed

- `obit.exe`, `obitd.exe` and `obit-gui.exe` in the release archives.
- They are built only from tagged commits of this repository, automatically, by the release workflow on GitHub Actions ([`release.yml`](https://github.com/woodfishhhh/obituary/blob/main/.github/workflows/release.yml)); the build scripts and CI configuration live in the repository.
- Every signing request is approved by hand by an approver listed below.
- The programs carry the product name "Obituary 讣告" and the version, shown under Properties → Details.

## Team roles

| Role | Members |
| --- | --- |
| Committers and reviewers | [woodfishhhh](https://github.com/woodfishhhh) |
| Approvers | [woodfishhhh](https://github.com/woodfishhhh) |

Every change from anyone outside the team is reviewed by a maintainer before it is merged. All members use multi-factor authentication on GitHub.

## Privacy

Obituary does one thing on the network: it checks for a new version at most once a day, and that can be turned off. See the [privacy policy](privacy).
