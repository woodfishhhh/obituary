Obituary — the trash bin for AI agents

This archive contains obit and its companion obitd for the operating system
and architecture in the archive name. Keep both executables together.

Verify the archive against checksums.txt before extracting it. Add the
installation directory to PATH, then run obit --version and obit doctor.
Run obit init -g --agent claude,codex --dry-run to review agent integration.
Run obit daemon enable --dry-run to review the user scheduler (launchd on
macOS, systemd --user on Linux). Linux without systemd can schedule
obitd --once with an absolute executable path in the user's crontab.

Files are buried in a graveyard and can be restored with obit resurrect.
The default retention period is 7 days. Recycle Bin mode and desktop
integration are available on Windows.

Documentation: https://obituary.woodfish.site/docs/en/
License: MIT, see LICENSE.
