// RGP2-025: the package type-checks under `reactogenic check` — components,
// behaviours, tests and the example site, which uses every component — and
// the contract's types reject what the spec says they reject.
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { expect, test } from "vitest";

const root = resolve(import.meta.dirname, "..");

function check(project: string) {
  const result = spawnSync(process.env.REACTOGENIC_BINARY!, ["check", "-p", project, "--pretty=false"], { encoding: "utf8" });
  return { status: result.status, output: result.stdout + result.stderr };
}

test("reactogenic check: exit 0, no diagnostics", () => {
  expect(check(root)).toEqual({ status: 0, output: "" });
});

test("the contract is typed: an option is a closed union, $Title is required, a command is a dialog's", () => {
  // Inside the package, so that its imports resolve as the site's do; outside the tsconfig's `include`.
  const dir = mkdtempSync(join(root, "bad-"));
  try {
    writeFileSync(join(dir, "tsconfig.json"), JSON.stringify({ extends: "../tsconfig.json", include: ["."] }));
    writeFileSync(
      join(dir, "bad.rtsx"),
      [
        `import { Button, Dialog, DropdownMenu } from "@reactogenic/ui";`,
        `export const a = (`,
        `  <DropdownMenu align="middle">`,
        `    <$Trigger>x</$Trigger>`,
        `    <$Item key="a" href="/">a</$Item>`,
        `  </DropdownMenu>`,
        `);`,
        `export const b = <Dialog>body</Dialog>;`,
        `export const c = <Button command="toggle-popover" commandfor="x">x</Button>;`,
        `export const d = <button command="toggle-popover" commandfor="x">the augmentation has every HTML command</button>;`,
        ``,
      ].join("\n"),
    );
    const { status, output } = check(dir);
    expect(status).toBe(1);
    const lines = output.split("\n").filter((line) => /bad\.rtsx\(\d+,\d+\): error/.test(line)); // without the related lines
    expect(lines).toHaveLength(3);
    expect(lines[0]).toMatch(/bad\.rtsx\(3,17\): error TS2322: .*"middle"/);
    expect(lines[1]).toMatch(/bad\.rtsx\(8,19\): error missing-slot: .*\$Title/);
    expect(lines[2]).toMatch(/bad\.rtsx\(9,26\): error TS2322: .*"toggle-popover"/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
