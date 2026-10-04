// Packages the extension: one .vsix per platform, each with its binary and
// the licences of the code in it, plus a universal one without a binary
// (ide.md, *Packaging*). Nothing is published.
//
//   node scripts/package.mjs [--pre-release] [target ...]
//
// Targets: darwin-arm64 darwin-x64 linux-arm64 linux-x64 win32-arm64 win32-x64
// and `universal`; none given means all seven. The binary of a platform is
// dist/npm/cli-<target>/bin/reactogenic[.exe] of the repository, as built by
// scripts/build-binaries.sh — the same file that goes to npm, never rebuilt.
// Output: dist/vsix/rtsx-<target>-<version>.vsix; vsce lists its files.
//
// Run it on macOS or Linux: a .vsix made on Windows loses the executable bit.
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { createVSIX } from "@vscode/vsce";
import { build, PLUGIN, root } from "./build.mjs";

export const PLATFORMS = ["darwin-arm64", "darwin-x64", "linux-arm64", "linux-x64", "win32-arm64", "win32-x64"];
const repo = path.resolve(root, "../..");
const out = path.join(repo, "dist/vsix");

/** What every .vsix holds; paths are the same in the package and in the .vsix. */
const FILES = [
  "README.md",
  "CHANGELOG.md",
  "icon.png",
  "language-configuration.json",
  "tags-language-configuration.json",
  "syntaxes/rtsx.tmLanguage.json",
  "syntaxes/rtsx.markdown.tmLanguage.json",
  "snippets/typescript.code-snippets",
  "dist/extension.js",
];
/** The TS server plugin's package (scripts/build.mjs), under node_modules/<its name> in both. */
const PLUGIN_FILES = ["package.json", "index.js"];

const args = process.argv.slice(2);
const preRelease = args.includes("--pre-release");
const targets = args.filter((a) => !a.startsWith("--"));
for (const target of targets) {
  if (target !== "universal" && !PLATFORMS.includes(target)) {
    throw new Error(`unknown target ${target}: one of ${[...PLATFORMS, "universal"].join(", ")}`);
  }
}
if (process.platform === "win32") {
  throw new Error("package on macOS or Linux: a .vsix made on Windows loses the binary's executable bit");
}

const manifest = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8"));
const bytes = await build({ production: true });
console.log(`dist/extension.js ${(bytes / 1024).toFixed(1)} KiB`);
fs.mkdirSync(out, { recursive: true });
const made = [];

for (const target of targets.length ? targets : [...PLATFORMS, "universal"]) {
  // A clean folder per .vsix: what is in it is what ships.
  const stage = path.join(root, "dist/stage", target);
  fs.rmSync(stage, { recursive: true, force: true });
  for (const file of FILES) {
    copy(path.join(root, file), path.join(stage, file));
  }
  // Ours (MIT); the grammar's and the bundle's (dist/ThirdPartyNotices.txt).
  copy(path.join(repo, "LICENSE"), path.join(stage, "LICENSE"));
  copy(path.join(root, "dist/ThirdPartyNotices.txt"), path.join(stage, "ThirdPartyNotices.txt"));
  // The TS server plugin, where tsserver looks for it by name: a package in
  // the extension's node_modules. It is the one dependency of the shipped
  // manifest, which is how vsce takes a folder under node_modules.
  const plugin = `node_modules/${PLUGIN}`;
  for (const file of PLUGIN_FILES) {
    copy(path.join(root, plugin, file), path.join(stage, plugin, file));
  }
  const shipped = {
    ...manifest,
    license: "MIT",
    files: [...FILES, "LICENSE", "ThirdPartyNotices.txt", ...PLUGIN_FILES.map((file) => `${plugin}/${file}`)],
    dependencies: { [PLUGIN]: manifest.version },
  };
  delete shipped["//"];
  delete shipped.scripts; // nothing to run at package time
  delete shipped.devDependencies; // all of it is in the bundle
  if (target !== "universal") {
    const exe = target.startsWith("win32-") ? "reactogenic.exe" : "reactogenic";
    const binary = path.join(repo, "dist/npm", `cli-${target}`, "bin", exe);
    if (!fs.existsSync(binary)) {
      throw new Error(`no binary for ${target}: run scripts/build-binaries.sh <version> ${target} (looked for ${binary})`);
    }
    copy(binary, path.join(stage, "server", exe));
    fs.chmodSync(path.join(stage, "server", exe), 0o755);
    // The binary holds tsgo (Apache-2.0) as well as our code: as in @reactogenic/cli-<target>.
    copy(path.join(repo, "go/third_party/tsgo/LICENSE"), path.join(stage, "LICENSE-typescript-go"));
    copy(path.join(repo, "go/third_party/tsgo/NOTICE.txt"), path.join(stage, "NOTICE-typescript-go.txt"));
    shipped.license = "MIT AND Apache-2.0";
    shipped.files.push(`server/${exe}`, "LICENSE-typescript-go", "NOTICE-typescript-go.txt");
  }
  fs.writeFileSync(path.join(stage, "package.json"), `${JSON.stringify(shipped, null, 2)}\n`);

  const packagePath = path.join(out, `rtsx-${target}-${manifest.version}.vsix`);
  await createVSIX({
    cwd: stage,
    packagePath,
    target: target === "universal" ? undefined : target,
    preRelease,
    // The extension's own dependencies are in its bundle; the staged manifest
    // names the plugin alone, and `npm list` in the stage finds its folder.
    dependencies: true,
    useYarn: false,
  });
  const listed = execFileSync("unzip", ["-Z1", packagePath], { encoding: "utf8" }).split("\n");
  for (const file of PLUGIN_FILES) {
    if (!listed.includes(`extension/${plugin}/${file}`)) {
      throw new Error(`${packagePath} does not hold ${plugin}/${file}: tsserver would not find the plugin`);
    }
  }
  made.push(`${path.relative(repo, packagePath)}  ${(fs.statSync(packagePath).size / 1024 / 1024).toFixed(2)} MB${preRelease ? "  pre-release" : ""}`);
}
console.log(`\n${made.join("\n")}`);

function copy(from, to) {
  fs.mkdirSync(path.dirname(to), { recursive: true });
  fs.copyFileSync(from, to);
}
