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

// Checks `source` as a project of its own: inside the package, so that its
// imports resolve as the site's do; outside the tsconfig's `include`. Returns
// the error lines of bad.rtsx, without their related lines.
function diagnose(source: string[]) {
  const dir = mkdtempSync(join(root, "bad-"));
  try {
    writeFileSync(join(dir, "tsconfig.json"), JSON.stringify({ extends: "../tsconfig.json", include: ["."] }));
    writeFileSync(join(dir, "bad.rtsx"), [...source, ""].join("\n"));
    const { status, output } = check(dir);
    return { status, output, lines: output.split("\n").filter((line) => /bad\.rtsx\(\d+,\d+\): error/.test(line)) };
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

test("reactogenic check: exit 0, no diagnostics", () => {
  expect(check(root)).toEqual({ status: 0, output: "" });
});

test("the contract is typed: an option is a closed union, $Title is required, a command is a dialog's, a side menu's cases exclude each other", () => {
  const { status, output, lines } = diagnose([
    `import { Button, Dialog, DropdownMenu, SideMenu } from "@reactogenic/ui";`,
    `export const a = (`,
    `  <DropdownMenu align="middle">`,
    `    <$Trigger>x</$Trigger>`,
    `    <$Item key="a" href="/">a</$Item>`,
    `  </DropdownMenu>`,
    `);`,
    `export const b = <Dialog>body</Dialog>;`,
    `export const c = <Button command="toggle-popover" commandfor="x">x</Button>;`,
    `export const d = <button command="toggle-popover" commandfor="x">the augmentation has every HTML command</button>;`,
    `export const e = (`,
    `  <SideMenu label="x">`,
    `    <$Section key="untitled" collapsed>`, // a collapsible section is a disclosure: it needs a title to open it
    `      <$Item key="a" href="/a/">a</$Item>`,
    `    </$Section>`,
    `    <$Section key="both" title="Both">`,
    `      <$Item key="b" href="/b/">`, // a link, or a disclosure of nested items: never both
    `        b`,
    `        <$Item key="c" href="/c/">c</$Item>`,
    `      </$Item>`,
    `    </$Section>`,
    `    <$Section key="fine" title="Fine" collapsed={false}>`,
    `      <$Item key="d">`,
    `        d`,
    `        <$Item key="e" href="/e/">e</$Item>`,
    `      </$Item>`,
    `    </$Section>`,
    `  </SideMenu>`,
    `);`,
  ]);
  expect(status).toBe(1);
  expect(lines).toHaveLength(5);
  expect(lines[0]).toMatch(/bad\.rtsx\(3,17\): error TS2322: .*"middle"/);
  expect(lines[1]).toMatch(/bad\.rtsx\(8,19\): error missing-slot: .*\$Title/);
  expect(lines[2]).toMatch(/bad\.rtsx\(9,26\): error TS2322: .*"toggle-popover"/);
  expect(lines[3]).toMatch(/bad\.rtsx\(13,19\): error TS2322: /);
  expect(output).toMatch(/Property 'title' is missing .* required in type 'SideMenuCollapsibleSectionProps'/);
  expect(lines[4]).toMatch(/bad\.rtsx\(17,22\): error TS2353: .*'href' does not exist in type 'SideMenuGroupProps'/);
});

// components.md: *Button* (a link takes nothing of a button; `Command`), rule 2
// (a slot's contract omits what the component wires), *DropdownMenu* (an item
// is a link or an action). One mistake per line, so the line names it.
test("the contract excludes what has no HTML: a link with a command, a trigger that is not the component's, a command the floor lacks", () => {
  const { lines } = diagnose([
    `import { Button, Dialog, DropdownMenu, SideMenu } from "@reactogenic/ui";`,
    `export const a = <Button href="/" command="show-modal" commandfor="x">a link commands nothing</Button>;`,
    `export const b = <Button href="/" popoverTarget="x">a link opens no popover</Button>;`,
    `export const c = <Button command="request-close" commandfor="x">not at the floor (Chrome 139)</Button>;`,
    `export const d = <Dialog><$Title>t</$Title><$Trigger href="/x">a link opens no dialog</$Trigger></Dialog>;`,
    `export const e = <Dialog><$Title>t</$Title><$Trigger commandfor="other">the dialog wires it</$Trigger></Dialog>;`,
    `export const f = <Dialog><$Title>t</$Title><$Action key="a" command="show-modal" commandfor="x">an action closes</$Action></Dialog>;`,
    `export const g = <DropdownMenu><$Trigger popoverTarget="x">the menu wires it</$Trigger><$Item key="a" href="/">a</$Item></DropdownMenu>;`,
    `export const h = <DropdownMenu><$Trigger>t</$Trigger><$Item key="a" href="/" command="show-modal" commandfor="x">a</$Item></DropdownMenu>;`,
    `export const i = <DropdownMenu><$Trigger>t</$Trigger><$Item key="a" command="request-close" commandfor="x">a</$Item></DropdownMenu>;`,
    `export const j = <SideMenu label="x"><$Toggle popoverTarget="y">Menu</$Toggle><$Section key="s"><$Item key="a" href="/">a</$Item></$Section></SideMenu>;`,
    `export const k = <SideMenu label="x"><$Toggle type="submit">Menu</$Toggle><$Section key="s"><$Item key="a" href="/">a</$Item></$Section></SideMenu>;`,
    // What stays legal: a disabled link, an action that is a link, the author's id, variant and label on a trigger and a toggle.
    `export const z = (`,
    `  <>`,
    `    <Button href="/" disabled aria-label="x">x</Button>`,
    `    <Button id="b" variant="ghost" command="close" commandfor="x" disabled>x</Button>`,
    `    <Button popoverTarget="x" popoverTargetAction="hide" aria-haspopup="menu">x</Button>`,
    `    <Dialog><$Title id="t">t</$Title><$Trigger id="x" variant="ghost" aria-label="x" disabled>x</$Trigger><$Action key="a" href="/">a</$Action><$Action key="b" variant="ghost" id="b">b</$Action></Dialog>`,
    `    <DropdownMenu><$Trigger id="y" variant="ghost">t</$Trigger><$Item key="a" command="close" commandfor="x" current>a</$Item><$Item key="b" href="/" disabled>b</$Item><$Item key="c" disabled>c</$Item></DropdownMenu>`,
    `    <SideMenu label="x"><$Toggle id="z" className="mine" aria-label="Menu">☰</$Toggle><$Section key="s"><$Item key="a" href="/">a</$Item></$Section></SideMenu>`,
    `  </>`,
    `);`,
  ]);
  // Every mistake where it is written — at the attribute, but for a link's
  // `popoverTarget` (the element) — and nothing in what is legal.
  const at = lines.map((line) => /bad\.rtsx\((\d+,\d+)\): error ([\w-]+): /.exec(line)!.slice(1).join(" "));
  expect(at).toEqual([
    "2,35 TS2322", // command, on a link
    "3,19 TS2322",
    "4,26 TS2322",
    "5,54 slot-type", // href, on the dialog's $Trigger
    "6,54 slot-type", // commandfor
    "7,61 slot-type", // command, on an $Action
    "7,82 slot-type", // … and its commandfor
    "8,42 slot-type", // popoverTarget, on the menu's $Trigger
    "9,78 slot-type", // command, on an item with href
    "10,69 slot-type",
    "11,47 TS2353",
    "12,47 TS2353",
  ]);
  expect(lines[1]).toMatch(/Type '\{ href: string; popoverTarget: string; children: string; \}' is not assignable to type 'IntrinsicAttributes & ButtonProps'/);
  expect(lines[2]).toMatch(/Type '"request-close"' is not assignable to type 'Command \| undefined'/);
  expect(lines[3]).toMatch(/`\$Trigger` does not match its declaration in `Dialog`: Type 'string' is not assignable to type 'undefined'/);
  expect(lines[7]).toMatch(/`\$Trigger` does not match its declaration in `DropdownMenu`: Type 'string' is not assignable to type 'undefined'/);
  expect(lines[9]).toMatch(/`\$Item` does not match its declaration in `DropdownMenu`: Type '"request-close"' is not assignable to type 'Command \| undefined'/);
  expect(lines[10]).toMatch(/'popoverTarget' does not exist in type 'SideMenuToggleProps'/);
  expect(lines[11]).toMatch(/'type' does not exist in type 'SideMenuToggleProps'/);
});
