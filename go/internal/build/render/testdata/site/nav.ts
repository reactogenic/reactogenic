export interface NavItem {
  href: string;
  label: string;
}

export const NAV: readonly NavItem[] = [
  { href: "/", label: "Introduction" },
  { href: "/guide/", label: "Guide" },
  { href: "/reference/cli/", label: "CLI" },
];
