// The sandbox of shell code (specs/phase02/builder.md, *Shell code in phase
// 2*, S4): what would make two builds of one page differ throws, or is
// pinned. The first module of the render bundle, so it is in place before
// any page's module runs. An error's name is its diagnostic code.
function coded(name, message) {
  const error = new Error(message);
  error.name = name;
  return error;
}

function nondeterministic(what) {
  return function () {
    throw coded("shell-nondeterministic", what + " makes the shell irreproducible");
  };
}

function define(name, value) {
  // A plain assignment fails where the global is an accessor (Node).
  Object.defineProperty(globalThis, name, { value, writable: true, configurable: true });
}

// `Date` itself stays: a date that is given is a compile-time value — in
// UTC, the shell's time zone on every machine. What is given in local terms
// (`new Date(2026, 9, 4)`, a date-time string without a zone) and what is
// read in them (`getHours()`, `toString()`) is UTC's.
const RealDate = Date;
const dates = RealDate.prototype;
const now = nondeterministic("Date.now()");
const noArguments = nondeterministic("new Date()");

// The formats the standard defines: ISO 8601 as ECMAScript has it — a time
// without a zone is local: UTC here — and what `toString()` and
// `toUTCString()` write. Anything else is parsed as its engine likes, and in
// the machine's zone.
const iso = /^([+-]\d{6}|\d{4})(-\d\d(-\d\d)?)?(T\d\d:\d\d(:\d\d(\.\d+)?)?(Z|[+-]\d\d:\d\d)?)?$/;
const written = /^[A-Z][a-z]{2},? [A-Z0-9][a-z0-9]{1,2} [A-Z0-9][a-z0-9]{1,2} -?\d{4,} \d\d:\d\d:\d\d GMT([+-]\d{4})?( \(.+\))?$/;

function parse(text) {
  text = String(text);
  const match = iso.exec(text);
  if (match !== null) return RealDate.parse(match[4] !== undefined && match[7] === undefined ? text + "Z" : text);
  if (written.test(text)) return RealDate.parse(text.replace(/ \(.+\)$/, ""));
  return nondeterministic("A date string that is not ISO 8601 (" + JSON.stringify(text) + ")")();
}

// ToPrimitive, as `new Date(value)` does it.
function primitive(value) {
  if (typeof value !== "object" || value === null) return value;
  if (value instanceof RealDate) return RealDate.prototype.getTime.call(value);
  const exotic = value[Symbol.toPrimitive];
  if (exotic != null) return exotic.call(value, "default");
  const number = value.valueOf();
  return typeof number === "object" && number !== null ? value.toString() : number;
}

const SandboxedDate = new Proxy(RealDate, {
  construct(target, args, newTarget) {
    if (args.length === 0) noArguments();
    if (args.length === 1) {
      const value = primitive(args[0]);
      args = [typeof value === "string" ? parse(value) : value];
    } else {
      args = [RealDate.UTC(...args)];
    }
    return Reflect.construct(target, args, newTarget === SandboxedDate ? target : newTarget);
  },
  apply: nondeterministic("Date()"),
  get(target, key) {
    return key === "now" ? now : key === "parse" ? parse : target[key];
  },
});
dates.constructor = SandboxedDate;
define("Date", SandboxedDate);

for (const field of ["FullYear", "Month", "Date", "Day", "Hours", "Minutes", "Seconds", "Milliseconds"]) {
  dates["get" + field] = dates["getUTC" + field];
  if (field !== "Day") dates["set" + field] = dates["setUTC" + field];
}
dates.getTimezoneOffset = function getTimezoneOffset() {
  return isNaN(this.getTime()) ? NaN : 0;
};
dates.getYear = function getYear() {
  return this.getUTCFullYear() - 1900;
};
dates.setYear = function setYear(year) {
  const y = Math.trunc(Number(year));
  return this.setUTCFullYear(y >= 0 && y <= 99 ? 1900 + y : Number(year));
};

// As V8 writes a date under TZ=UTC.
const DAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const two = (n) => String(n).padStart(2, "0");

function day(date) {
  const year = date.getUTCFullYear();
  return (
    DAYS[date.getUTCDay()] + " " + MONTHS[date.getUTCMonth()] + " " + two(date.getUTCDate()) + " " +
    (year < 0 ? "-" : "") + String(Math.abs(year)).padStart(4, "0")
  );
}

function time(date) {
  return (
    two(date.getUTCHours()) + ":" + two(date.getUTCMinutes()) + ":" + two(date.getUTCSeconds()) +
    " GMT+0000 (Coordinated Universal Time)"
  );
}

function valid(date) {
  return !isNaN(RealDate.prototype.getTime.call(date)); // a TypeError for what is not a date, as the method's own
}

dates.toString = function toString() {
  return valid(this) ? day(this) + " " + time(this) : "Invalid Date";
};
dates.toDateString = function toDateString() {
  return valid(this) ? day(this) : "Invalid Date";
};
dates.toTimeString = function toTimeString() {
  return valid(this) ? time(this) : "Invalid Date";
};

Math.random = nondeterministic("Math.random()");
define("performance", { now: nondeterministic("performance.now()") });
define("crypto", {
  getRandomValues: nondeterministic("crypto.getRandomValues()"),
  randomUUID: nondeterministic("crypto.randomUUID()"),
});

// When an object is freed is the collector's business — the engine's, and
// what else was allocated: not there, as the timers are not.
delete globalThis.WeakRef;
delete globalThis.FinalizationRegistry;

// The engine has no Intl, and its locale-sensitive methods answer without
// one — a number unformatted, strings compared by code unit: not what the
// same code gives in a browser. They throw, as `Intl` itself does by not
// being there.
function needsIntl(prototype, type, names) {
  for (const name of names) {
    prototype[name] = function () {
      throw coded("shell-error", type + ".prototype." + name + "() needs Intl, which the builder's engine does not have");
    };
  }
}
needsIntl(Number.prototype, "Number", ["toLocaleString"]);
needsIntl(BigInt.prototype, "BigInt", ["toLocaleString"]);
needsIntl(dates, "Date", ["toLocaleString", "toLocaleDateString", "toLocaleTimeString"]);
needsIntl(String.prototype, "String", ["localeCompare", "toLocaleUpperCase", "toLocaleLowerCase"]);

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
