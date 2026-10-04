// Checks a packaged .vsix as it ships (plan.md, RGP1-113): what is in it,
// and — for the platform this runs on — that its bundled binary starts and
// answers `initialize`.
//
//   node scripts/smoke-vsix.mjs [--run] dist/vsix/rtsx-darwin-arm64-0.1.1.vsix [...]
//
// A .vsix of another platform is checked for its files only — with `--run`
// that is an error (CI: one runner per platform); the universal one must
// hold no binary.
import { execFileSync, spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const PLUGIN = "node_modules/reactogenic-typescript-plugin";
// As vsce names them: the page, the changelog and the licence are renamed.
const ALWAYS = [
  "package.json",
  "readme.md",
  "changelog.md",
  "LICENSE.txt",
  "icon.png",
  "ThirdPartyNotices.txt",
  "dist/extension.js",
  "syntaxes/rtsx.tmLanguage.json",
  `${PLUGIN}/package.json`,
  `${PLUGIN}/index.js`,
];
const here = `${process.platform}-${process.arch}`;

const run = process.argv.includes("--run");
const files = process.argv.slice(2).filter((arg) => arg !== "--run");
if (files.length === 0) {
  throw new Error("usage: smoke-vsix.mjs [--run] file.vsix [...]");
}
for (const file of files) {
  const target = /rtsx-(.+)-\d+\.\d+\.\d+\.vsix$/.exec(path.basename(file))?.[1];
  if (!target) {
    throw new Error(`${file}: not rtsx-<target>-<version>.vsix`);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "rtsx-vsix-"));
  try {
    // Windows' own tar (bsdtar) reads zip. By its path: under Git Bash `tar`
    // is GNU tar, which reads no zip and takes `C:` for a host.
    if (process.platform === "win32") {
      execFileSync(path.join(process.env.SystemRoot ?? "C:\\Windows", "System32", "tar.exe"), ["-xf", path.resolve(file)], { cwd: dir });
    } else {
      execFileSync("unzip", ["-q", path.resolve(file), "-d", dir]);
    }
    const extension = path.join(dir, "extension");
    for (const name of ALWAYS) {
      if (!fs.existsSync(path.join(extension, name))) {
        throw new Error(`${file}: no ${name}`);
      }
    }
    const manifest = JSON.parse(fs.readFileSync(path.join(extension, "package.json"), "utf8"));
    if (target === "universal") {
      if (fs.existsSync(path.join(extension, "server"))) {
        throw new Error(`${file}: the universal package holds a binary`);
      }
      console.log(`${path.basename(file)}: files`);
      continue;
    }
    const exe = path.join(extension, "server", target.startsWith("win32-") ? "reactogenic.exe" : "reactogenic");
    for (const name of [exe, path.join(extension, "LICENSE-typescript-go"), path.join(extension, "NOTICE-typescript-go.txt")]) {
      if (!fs.existsSync(name)) {
        throw new Error(`${file}: no ${path.relative(extension, name)}`);
      }
    }
    if (target !== here && run) {
      throw new Error(`${file}: this is ${here}, its binary cannot be run here`);
    }
    if (target !== here) {
      console.log(`${path.basename(file)}: files (its binary is not run on ${here})`);
      continue;
    }
    if (process.platform !== "win32" && !(fs.statSync(exe).mode & 0o100)) {
      throw new Error(`${file}: server/reactogenic is not executable`);
    }
    const info = await initialize(exe, dir);
    console.log(`${path.basename(file)}: files; ${info.name} ${info.version} answers initialize (extension ${manifest.version})`);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

/** Starts `exe lsp --stdio`, as the extension does, and ends it as an editor ends a server. */
function initialize(exe, cwd) {
  return new Promise((resolve, reject) => {
    const child = spawn(exe, ["lsp", "--stdio"], { cwd, stdio: ["pipe", "pipe", "pipe"] });
    const timer = setTimeout(() => {
      child.kill();
      reject(new Error(`${exe}: no answer to initialize within 10 s${stderr ? `\n${stderr}` : ""}`));
    }, 10_000);
    let stderr = "";
    let buffer = Buffer.alloc(0);
    let info;
    const send = (message) => {
      const body = JSON.stringify({ jsonrpc: "2.0", ...message });
      child.stdin.write(`Content-Length: ${Buffer.byteLength(body)}\r\n\r\n${body}`);
    };
    child.stderr.on("data", (chunk) => {
      stderr += chunk;
    });
    child.on("error", (error) => {
      clearTimeout(timer);
      reject(error);
    });
    child.on("exit", (code) => {
      clearTimeout(timer);
      if (info && code === 0) {
        resolve(info);
      } else {
        reject(new Error(`${exe}: exit status ${code}${info ? " after shutdown" : " before it answered"}${stderr ? `\n${stderr}` : ""}`));
      }
    });
    child.stdout.on("data", (chunk) => {
      buffer = Buffer.concat([buffer, chunk]);
      for (;;) {
        const head = buffer.indexOf("\r\n\r\n");
        const length = head < 0 ? undefined : /Content-Length: (\d+)/i.exec(buffer.subarray(0, head).toString())?.[1];
        if (length === undefined || buffer.length < head + 4 + Number(length)) {
          return;
        }
        const message = JSON.parse(buffer.subarray(head + 4, head + 4 + Number(length)).toString());
        buffer = buffer.subarray(head + 4 + Number(length));
        if (message.id === 1) {
          if (!message.result?.capabilities || !message.result.serverInfo) {
            child.kill();
            reject(new Error(`${exe}: initialize answered ${JSON.stringify(message).slice(0, 300)}`));
            return;
          }
          info = message.result.serverInfo;
          send({ method: "initialized", params: {} });
          send({ id: 2, method: "shutdown" });
        } else if (message.id === 2) {
          send({ method: "exit" });
        } else if (message.id !== undefined && message.method) {
          send({ id: message.id, result: null }); // a request of the server's
        }
      }
    });
    send({ id: 1, method: "initialize", params: { processId: process.pid, rootUri: null, capabilities: {} } });
  });
}
