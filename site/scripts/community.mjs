// Fetch the contributor wall and the star charts at build time, so the built
// site never loads them from GitHub or anyone else at runtime. Everything
// here is optional: on any failure (no network, private repository, rate
// limit) the page simply leaves the block out, and the build still passes.
// Output (git-ignored): landing/public/community/ and
// landing/src/generated/community.json.
import { mkdir, rm, writeFile } from "node:fs/promises";

const repo = "woodfishhhh/obituary";
const root = new URL("../landing/", import.meta.url);
const publicDir = new URL("public/community/", root);
const dataFile = new URL("src/generated/community.json", root);
const minStars = 10;

const headers = { Accept: "application/vnd.github+json", "User-Agent": "obituary-site-build" };
if (process.env.GITHUB_TOKEN) headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;

async function get(url, init = {}) {
  const res = await fetch(url, { ...init, signal: AbortSignal.timeout(15000) });
  if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
  return res;
}

async function download(url, name) {
  const res = await get(url);
  const type = res.headers.get("content-type") ?? "";
  const ext = type.includes("svg") ? "svg" : type.includes("jpeg") ? "jpg" : type.includes("webp") ? "webp" : "png";
  const file = `${name}.${ext}`;
  await writeFile(new URL(file, publicDir), Buffer.from(await res.arrayBuffer()));
  return `community/${file}`;
}

const data = { stars: 0, contributors: [], starHistory: null, starMap: null };
await rm(publicDir, { recursive: true, force: true });
await mkdir(new URL("avatars/", publicDir), { recursive: true });
await mkdir(new URL("src/generated/", root), { recursive: true });

try {
  const info = await (await get(`https://api.github.com/repos/${repo}`, { headers })).json();
  data.stars = info.stargazers_count ?? 0;
  const people = await (await get(`https://api.github.com/repos/${repo}/contributors?per_page=100`, { headers })).json();
  for (const p of people.filter((x) => x.type === "User")) {
    try {
      const avatar = await download(`${p.avatar_url}${p.avatar_url.includes("?") ? "&" : "?"}s=96`, `avatars/${p.login}`);
      data.contributors.push({ login: p.login, url: p.html_url, avatar, contributions: p.contributions });
    } catch (e) {
      console.warn(`community: skipped avatar of ${p.login}: ${e.message}`);
    }
  }
} catch (e) {
  console.warn(`community: no repository data (${e.message}); the page leaves the wall empty`);
}

if (data.stars >= minStars) {
  try {
    data.starHistory = await download(`https://api.star-history.com/svg?repos=${repo}&type=Date`, "star-history");
  } catch (e) {
    console.warn(`community: no star history (${e.message})`);
  }
  try {
    data.starMap = await download(`https://starmapper.bruniaux.com/api/map-image/${repo}`, "star-map");
  } catch (e) {
    console.warn(`community: no star map (${e.message})`);
  }
}

await writeFile(dataFile, `${JSON.stringify(data, null, 2)}\n`);
console.log(`community: ${data.contributors.length} contributors, ${data.stars} stars`);
