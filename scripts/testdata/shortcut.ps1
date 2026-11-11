param([string]$Path, [string]$ReplaceTarget)
$ErrorActionPreference = 'Stop'
# WScript.Shell cannot load or save a .lnk whose name has characters outside
# the system ANSI code page (the Chinese name on English Windows), so it works
# on an ASCII-named copy and writes changes back under the real name.
$work = Join-Path ([IO.Path]::GetTempPath()) ('obit-test-shortcut-' + [guid]::NewGuid().ToString('N') + '.lnk')
try {
    if (Test-Path -LiteralPath $Path) { [IO.File]::Copy($Path, $work) }
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($work)
    if ($ReplaceTarget) {
        $shortcut.TargetPath = $ReplaceTarget
        $shortcut.Description = 'User shortcut'
        $shortcut.Save()
        [IO.File]::Copy($work, $Path, $true)
    }
    @{ target = $shortcut.TargetPath; icon = $shortcut.IconLocation; workingDirectory = $shortcut.WorkingDirectory } | ConvertTo-Json -Compress
}
finally {
    if (Test-Path -LiteralPath $work) { [IO.File]::Delete($work) }
}
