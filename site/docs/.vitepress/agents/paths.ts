import { initCommand, initPrefix } from '../../../landing/src/agents-data';
import { guides } from './catalog';

export type Copy = {
  intro: string; setup: string; project: string; global: string; files: string; variant: string;
  check: string; test: string; remove: string; pause: string; disconnect: string; manualOff: string;
  notes: Record<string, string>;
};

const code = (value: string) => '\n```sh\n' + value + '\n```\n';

export function agentPaths(t: Copy) {
  return { paths: () => guides.map((guide) => {
    const { id, name, paths, global, variant, manual } = guide;
    const base = initPrefix(guide);
    const install = (to: string) => initCommand(guide, to);
    const locations = (Array.isArray(paths) ? paths.map(p => `- \`${p}\``) :
      Object.entries(paths).map(([os, p]) => `- ${os}: \`${p}\``)).join('\n');
    const content = [
      `# ${name}`, t.intro, `## ${t.setup}`, global ? t.global : t.project, code(install(id)),
      ...(variant ? [t.variant.replace('{id}', `\`${variant}\``), code(install(variant))] : []),
      t.files, locations, t.notes[id],
      `## ${t.check}`, t.test, code(`${base}${id} --show`),
      `## ${t.remove}`, manual ? t.manualOff : t.pause, t.disconnect, code(`${base}${id} --uninstall`),
    ];
    if (!t.notes[id]) throw new Error(`Missing guide note for ${id}`);
    return { params: { id }, content: content.join('\n\n') };
  }) };
}
