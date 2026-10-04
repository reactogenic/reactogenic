// Runs a built script against a fake `document` and prints what it did:
//   node run.mjs <script.mjs> <pathname> <id,id,…>
// The listeners a script adds are recorded, then each is called — a key
// listener with an arrow and with a letter — so that what they do shows too.
const [script, pathname, ids] = process.argv.slice(2);

const added = []; // "m1:keydown", "window:pagehide", in the order they were added
const listeners = [];
const elements = new Map();
const popover = { attributes: {}, setAttribute(name, value) { this.attributes[name] = value; } };

function element(id) {
  if (!elements.has(id)) {
    const el = {
      id,
      dataset: { count: "3", at: "2" },
      addEventListener(type, listener) {
        added.push(`${id}:${type}`);
        listeners.push([el, type, listener]);
      },
    };
    elements.set(id, el);
  }
  return elements.get(id);
}

globalThis.document = {
  title: "",
  getElementById: (id) => (ids.split(",").includes(id) ? element(id) : null),
  querySelectorAll: () => [popover],
};
globalThis.location = { pathname };
globalThis.addEventListener = (type, listener) => {
  added.push(`window:${type}`);
  listeners.push([null, type, listener]);
};

await import(script);

for (const [el, type, listener] of listeners) {
  for (const key of type === "keydown" ? ["ArrowDown", "a"] : [""]) {
    listener({ type, key, currentTarget: el, target: element("clicked") });
  }
}

console.log(JSON.stringify({
  added,
  datasets: Object.fromEntries([...elements].map(([id, el]) => [id, el.dataset])),
  title: document.title,
  popover: popover.attributes,
}));
