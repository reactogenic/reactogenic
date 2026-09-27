// Finds the `reactogenic` binary for this platform (RGP1-092): the one in
// @reactogenic/cli-<platform>-<arch>, an optional dependency npm installs
// only where it fits — the esbuild model. $REACTOGENIC_BINARY overrides it.
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);

/**
 * The platform package for `platform` / `arch` (Node's names).
 * @param {string} [platform]
 * @param {string} [arch]
 */
export function platformPackage(platform = process.platform, arch = process.arch) {
  return `@reactogenic/cli-${platform}-${arch}`;
}

/**
 * The absolute path of the binary, or an Error saying why there is none.
 * @param {{ env?: NodeJS.ProcessEnv, platform?: string, arch?: string, resolve?: (id: string) => string }} [options]
 * @returns {string}
 */
export function binaryPath(options = {}) {
  const env = options.env ?? process.env;
  if (env.REACTOGENIC_BINARY) {
    return env.REACTOGENIC_BINARY;
  }
  const platform = options.platform ?? process.platform;
  const pkg = platformPackage(platform, options.arch ?? process.arch);
  const file = platform === "win32" ? "reactogenic.exe" : "reactogenic";
  try {
    return (options.resolve ?? require.resolve)(`${pkg}/bin/${file}`);
  } catch {
    throw new Error(
      `reactogenic: no binary for ${platform}-${options.arch ?? process.arch}: ${pkg} is not installed. ` +
        `Reinstall without --no-optional, or set REACTOGENIC_BINARY.`,
    );
  }
}
