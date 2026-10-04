// A component imports its CSS (`import "./dialog.css"`), and TypeScript 7
// checks side-effect imports too (TS2882). This file is a script, not a
// module, so the declaration is ambient: it covers every `.css` import of a
// program that holds @reactogenic/ui — the design system's and the site's.
declare module "*.css";
