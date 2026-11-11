import { spawn } from 'node:child_process';
import { readFileSync } from 'node:fs';

// Native MiniMax MCP packages require a PATH interpreter and relative arguments.
// The managed runtime file pins obit.exe without exposing a shell command.
const config = JSON.parse(readFileSync(new URL('./runtime.json', import.meta.url), 'utf8'));
const child = spawn(config.command, config.args, { shell: false, windowsHide: true, stdio: 'inherit' });
child.on('error', (error) => { console.error(error.message); process.exitCode = 1; });
child.on('exit', (code) => { process.exitCode = code ?? 1; });
for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, () => child.kill(signal));
