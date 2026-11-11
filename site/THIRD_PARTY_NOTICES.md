# Third-party notices

Licence notices for third-party material in `site/` and in the site it builds.

## Magic UI components (MIT)

These files were copied from Magic UI and adapted:

- `landing/src/components/ui/blur-fade.tsx`
- `landing/src/components/ui/border-beam.tsx`
- `landing/src/components/ui/file-tree.tsx`
- `landing/src/components/ui/sparkles-text.tsx`
- `landing/src/components/ui/terminal.tsx`
- `landing/src/components/ui/shimmer-link.tsx` (adapted from the shimmer button)

```
MIT License

Copyright (c) Magic UI

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## React Bits components (MIT + Commons Clause)

These files were copied from React Bits. They are covered by the
licence below, not by Obituary's MIT licence: they may be used and changed as
part of a website or product, but not sold or redistributed on their own.

- `landing/src/components/reactbits/CountUp.tsx`
- `landing/src/components/reactbits/DecryptedText.tsx`
- `landing/src/components/reactbits/ElectricBorder.tsx`
- `landing/src/components/reactbits/StarBorder.tsx`

```
MIT + Commons Clause License Condition v1.0

Copyright (c) 2026 David Haz

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, and distribute the Software **as part of an application, website, or product**, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

## Commons Clause Restriction

You may use this Software, including for any commercial purpose, **so long as you do not sell, sublicense, or redistribute the components themselves-whether alone, in a bundle, or as a ported version.**

The same restriction covers the React Bits tools (Background Studio, Texture Lab, Shape Magic, Center Stage, Sound Lab and any tools added later) and the sounds in the sound library. **You may not sell, sublicense, or redistribute the tools or the sounds themselves, or offer the tools or a substantially similar copy of them as a hosted service-whether alone, in a bundle, or as a ported version.**

What you make with the tools is yours to use. You can use exported code, images, videos, and audio files in any personal or commercial application, website, or product, including paid ones. **You may not sell or redistribute the exports on their own, either individually or as the product itself, such as a sound, texture, background, or template pack.**

## No Warranty

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## Agent marks (MIT)

The agent logos on the landing page (`landing/src/agents.ts`) are SVG files
from the `@lobehub/icons-static-svg` package (lobe-icons), used unaltered and
installed from npm at build time. Each mark remains the trademark of its owner;
it is shown only to say which agents Obituary can connect to.

| Agent | File |
|---|---|
| Claude Code | `claudecode-color.svg` |
| Codex | `codex-color.svg` |
| GitHub Copilot | `githubcopilot.svg` |
| Cursor | `cursor.svg` |
| Gemini CLI | `geminicli-color.svg` |
| Trae | `trae-color.svg` |
| Kimi | `kimi-color.svg` |
| Mistral Vibe | `mistral-color.svg` |
| Google Antigravity | `antigravity-color.svg` |
| CodeBuddy | `codebuddy-color.svg` |
| Qoder, QoderWork | `qoder-color.svg` |
| ZCode | `zhipu-color.svg` |
| OpenCode | `opencode.svg` |
| OpenClaw | `openclaw-color.svg` |
| Hermes | `hermesagent.svg` |
| Cline | `cline.svg` |
| Kilo Code | `kilocode.svg` |
| MiniMax Code | `minimax-color.svg` |
| DeepSeek Harness | `deepseek-color.svg` |
| Roo Code | `roocode.svg` |
| Windsurf | `windsurf.svg` |
| WorkBuddy | `workbuddy-color.svg` |

```
MIT License

Copyright (c) 2023 LobeHub
```

(The full MIT text is the same as above.)

## Agent marks from their vendors

Four agents have no mark in that package. Their own marks are in
`landing/src/assets/agents/`, used unaltered (Oh My Pi's light mark sits on a
dark tile; the file is not recoloured). Each is the trademark of its owner,
shown only to say that Obituary can connect to that agent (nominative use).

| Agent | File | Source |
|---|---|---|
| Pi | `pi.svg` | https://pi.dev/logo-auto.svg, the logo in the README of badlogic/pi-mono |
| Oh My Pi | `oh-my-pi.svg` | `assets/icon.svg` in the can1357/oh-my-pi repository |
| Factory Droid | `factory.svg` | the favicon of https://factory.ai |
| Lingma | `lingma.png` | the icon of the VS Code extension `Alibaba-Cloud.tongyi-lingma` |

## Fonts (SIL Open Font License 1.1)

The built landing page serves these fonts, installed from npm at build time:

- Geist and Geist Mono: Copyright 2024 The Geist Project Authors (https://github.com/vercel/geist-font).
- Noto Sans SC (a subset of the glyphs the page uses): Copyright Google Inc.

Both are licensed under the SIL Open Font License, Version 1.1:
https://openfontlicense.org/open-font-license-official-text/
