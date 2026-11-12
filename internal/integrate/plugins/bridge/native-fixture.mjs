// Fixture paths describe the host running the callback tests.
export function nativePath(value) {
  return process.platform === "win32" ? value : value.slice(2).replaceAll("\\", "/");
}
