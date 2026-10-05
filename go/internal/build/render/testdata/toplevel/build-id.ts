// Read when the module loads, before any page renders: no page can be built.
console.warn("build id");
export const BUILD_ID = Math.random().toString(36).slice(2);
