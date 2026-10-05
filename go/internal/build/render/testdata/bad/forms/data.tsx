// Not a page of the fixture: TestMountData renders it at a pathname per
// form — what a `mount()` may hand its behaviour, and what it may not.
type Build = { pathname: string; mount(module: string, id?: string, flags?: Record<string, boolean>, data?: unknown): void };
const build = () => (globalThis as unknown as { __reactogenic_build: Build }).__reactogenic_build;

const circle: { self?: unknown } = {};
circle.self = circle;

function Form() {
  const { pathname, mount } = build();
  switch (pathname) {
    case "/menu/":
      // Keys in any order, an `undefined` among them; no data; an empty object.
      mount("ui/keys", "m1", { RG_TYPE: true }, { typeahead: true, items: [1, "two", null, { b: false, a: 0.5 }], wrap: undefined, "data-x": "</script>" });
      mount("ui/keys", "m2");
      mount("ui/keys", "m3", undefined, {});
      mount("ui/page", undefined, undefined, null);
      break;
    case "/function/":
      mount("ui/keys", "m1", undefined, { onPick: () => {} });
      break;
    case "/page/":
      mount("ui/page", undefined, undefined, { typeahead: true });
      break;
    case "/array/":
      mount("ui/keys", "m1", undefined, ["typeahead"]);
      break;
    case "/string/":
      mount("ui/keys", "m1", undefined, "typeahead");
      break;
    case "/date/":
      mount("ui/keys", "m1", undefined, { since: [new Date(2026, 9, 5)] });
      break;
    case "/nan/":
      mount("ui/keys", "m1", undefined, { delay: 0 / 0 });
      break;
    case "/proto/":
      mount("ui/keys", "m1", undefined, JSON.parse('{ "a": { "__proto__": 1 } }'));
      break;
    case "/circle/":
      mount("ui/keys", "m1", undefined, { "the circle": circle });
      break;
  }
  return <b id="m1">{pathname}</b>;
}

export default function DataPage() {
  return (
    <html lang="en">
      <body>
        <Form />
      </body>
    </html>
  );
}
