// The sandbox of shell code (specs/phase02/builder.md, *Shell code in phase
// 2*, S4): what would make two builds of one page differ throws. The first
// module of the render bundle, so it is in place before any page's module
// runs. The error's name is its diagnostic code.
function nondeterministic(what) {
  return function () {
    const error = new Error(what + " makes the shell irreproducible");
    error.name = "shell-nondeterministic";
    throw error;
  };
}

function define(name, value) {
  // A plain assignment fails where the global is an accessor (Node).
  Object.defineProperty(globalThis, name, { value, writable: true, configurable: true });
}

// `Date` itself stays: a date with arguments is a compile-time value.
const RealDate = Date;
const now = nondeterministic("Date.now()");
const noArguments = nondeterministic("new Date()");
const SandboxedDate = new Proxy(RealDate, {
  construct(target, args, newTarget) {
    if (args.length === 0) noArguments();
    return Reflect.construct(target, args, newTarget === SandboxedDate ? target : newTarget);
  },
  apply: nondeterministic("Date()"),
  get(target, key) {
    return key === "now" ? now : target[key];
  },
});
RealDate.prototype.constructor = SandboxedDate;
define("Date", SandboxedDate);
Math.random = nondeterministic("Math.random()");
define("performance", { now: nondeterministic("performance.now()") });
define("crypto", {
  getRandomValues: nondeterministic("crypto.getRandomValues()"),
  randomUUID: nondeterministic("crypto.randomUUID()"),
});

// `console`: the engine has none. What shell code prints is collected, with
// where it was printed, and reported as warnings.
const messages = [];

function format(value) {
  if (typeof value === "string") return value;
  if (value instanceof Error) return value.name + ": " + value.message;
  try {
    const json = JSON.stringify(value);
    if (json !== undefined) return json;
  } catch {}
  return String(value);
}

const collected = {};
for (const level of ["log", "info", "warn", "error", "debug", "trace"]) {
  collected[level] = function (...args) {
    messages.push({ level, text: args.map(format).join(" "), stack: String(new Error().stack) });
  };
}
define("console", new Proxy(collected, { get: (target, key) => target[key] ?? function () {} }));

// What was printed since the last call, as JSON: the engine asks after the
// bundle has loaded and after each page.
define("__reactogenic_console", () => JSON.stringify(messages.splice(0)));
