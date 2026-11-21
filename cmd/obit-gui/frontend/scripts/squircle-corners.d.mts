// The plugin has no imports, so it carries no postcss types: a postcss plugin is any object like this one.
declare const squircleCorners: {
  (options?: { roundSelectors?: RegExp }): { postcssPlugin: string; Once(root: any): void };
  postcss: true;
};
export default squircleCorners;
