import { execSync } from 'node:child_process';
import { existsSync } from 'node:fs';

const frontend = new URL('../cmd/obit-gui/frontend/', import.meta.url);

// Transitional until the pnpm frontend lands; dist is already tracked.
if (!existsSync(new URL('package.json', frontend))) {
  console.log('No frontend/package.json; using the committed frontend/dist.');
} else {
  // Node's execSync supports pnpm.cmd on Windows; execFileSync does not.
  // Reference: nodejs.org/api/child_process.html#spawning-bat-and-cmd-files-on-windows
  execSync('pnpm install --frozen-lockfile && pnpm build', {
    cwd: frontend,
    stdio: 'inherit',
    windowsHide: true,
  });
}
