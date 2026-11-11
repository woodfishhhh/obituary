# Obituary exit-code transport v1. Fixed argv, no shell evaluation or host logic.
# System.Diagnostics.Process follows the existing Node execFile transport contract.
$ErrorActionPreference = 'Stop'
$code = 0
$child = $null
try {
    $utf8 = New-Object System.Text.UTF8Encoding($false, $true)
    [Console]::InputEncoding = $utf8
    [Console]::OutputEncoding = $utf8
    $buffer = New-Object char[] (4 * 1024 * 1024 + 1)
    $length = 0
    while ($length -lt $buffer.Length) {
        $read = [Console]::In.Read($buffer, $length, $buffer.Length - $length)
        if ($read -eq 0) { break }
        $length += $read
    }
    if ($length -gt 4 * 1024 * 1024) { throw 'input limit' }
    $payload = New-Object string($buffer, 0, $length)
    $info = New-Object System.Diagnostics.ProcessStartInfo
    $info.FileName = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('__OBIT_EXE_BASE64__'))
    $info.Arguments = 'hook __OBIT_AGENT__ --event PreToolUse'
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardInput = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    $info.StandardOutputEncoding = $utf8
    $info.StandardErrorEncoding = $utf8
    $child = New-Object System.Diagnostics.Process
    $child.StartInfo = $info
    if (-not $child.Start()) { throw 'start failed' }
    $stdout = $child.StandardOutput.ReadToEndAsync()
    $stderr = $child.StandardError.ReadToEndAsync()
    $write = $child.StandardInput.WriteAsync($payload)
    if (-not $write.Wait(2000)) { throw 'input timeout' }
    $child.StandardInput.Close()
    if (-not $child.WaitForExit(2000)) { throw 'process timeout' }
    if (-not $stdout.Wait(2000) -or -not $stderr.Wait(2000)) { throw 'output timeout' }
    if ($child.ExitCode -ne 0 -or $stderr.Result.Length -ne 0) { throw 'child failed' }
    $raw = $stdout.Result
    if ($utf8.GetByteCount($raw) -gt 16384) { throw 'output limit' }
    # Parse using the platform JSON parser, then require our exact compact
    # envelope. This shape check rejects duplicates, extra fields and trailing
    # output and invalid escapes (Windows PowerShell 5.1 accepts \' in JSON).
    $reply = ConvertFrom-Json -InputObject $raw -ErrorAction Stop
    $shape = '\A\{"version":1,"event":"PreToolUse","deny":true,"reason":"(?:[^"\\\x00-\x1f]|\\(?:["\\/bfnrt]|u[0-9A-Fa-f]{4}))*"\}\r?\n?\z'
    if ($raw -cmatch $shape -and $reply.reason -is [string] -and
        $reply.reason.Length -gt 0 -and $utf8.GetByteCount($reply.reason) -le 2000) {
        [Console]::Error.WriteLine($reply.reason)
        $code = 2
    }
} catch {
    # A transport failure must never become a deletion denial.
    $code = 0
} finally {
    if ($null -ne $child) {
        try {
            if (-not $child.HasExited) { $child.Kill(); $child.WaitForExit() }
            $child.Dispose()
        } catch { $code = 0 }
    }
}
exit $code
