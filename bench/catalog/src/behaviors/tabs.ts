// tabs — per tab set (bench/catalog/README.md: a measurement fixture, in the
// authoring style of specs/phase02/builder.md, *Behaviours*). Mounted by
// Tabs: the platform has no tabs, so selecting one — by a click, by the
// arrow keys, Home and End (APG, automatic activation) — is this script's.
//
// What it writes is state, named in full (builder.md, what a behaviour
// writes): `aria-selected` on the tabs, `tabIndex` on them, `hidden` on the
// panels. No class, no element.
declare const RG_TABS_HASH: boolean;

/** What a tab set hands its behaviour: the mount's data. */
export interface TabsData {
  /** The tab the URL's hash names is shown, and follows the hash. */
  hash?: boolean;
}

// The tab sets that follow the hash.
const hashed: HTMLElement[] = [];

export default function mountTabs(root: HTMLElement, own?: TabsData): void {
  const list = root.firstElementChild as HTMLElement;
  list.addEventListener("click", onClick);
  list.addEventListener("keydown", onKey);
  // The flag is the page's — whether this code is here; the data, this root's.
  if (RG_TABS_HASH && own?.hash) {
    if (!hashed.length) {
      addEventListener("hashchange", onHash);
    }
    hashed.push(list);
    fromHash(list);
  }
}

function tabsOf(list: HTMLElement): HTMLElement[] {
  return [...list.children] as HTMLElement[];
}

// Shows `tab`'s panel and no other of its list.
function select(list: HTMLElement, tab: HTMLElement): void {
  for (const each of tabsOf(list)) {
    const on = each === tab;
    each.setAttribute("aria-selected", on ? "true" : "false");
    each.tabIndex = on ? 0 : -1;
    document.getElementById(each.getAttribute("aria-controls")!)!.hidden = !on;
  }
}

function onClick(event: MouseEvent): void {
  const list = event.currentTarget as HTMLElement;
  const tab = (event.target as Element).closest<HTMLElement>("[role=tab]");
  if (tab && tab.parentElement === list) {
    select(list, tab);
  }
}

// Left and Right with wrap, Home, End: focus moves, and the tab is selected.
function onKey(event: KeyboardEvent): void {
  const list = event.currentTarget as HTMLElement;
  const tabs = tabsOf(list);
  const at = tabs.indexOf(event.target as HTMLElement);
  const to = ({ ArrowRight: at + 1, ArrowLeft: at - 1, Home: 0, End: -1 } as Record<string, number | undefined>)[event.key];
  if (at < 0 || to === undefined) {
    return;
  }
  event.preventDefault();
  const tab = tabs.at(to % tabs.length)!;
  tab.focus();
  select(list, tab);
}

function onHash(): void {
  hashed.forEach(fromHash);
}

// The tab whose id the hash is, if it is one of this list.
function fromHash(list: HTMLElement): void {
  const tab = tabsOf(list).find((each) => "#" + each.id === location.hash);
  if (tab) {
    select(list, tab);
  }
}
