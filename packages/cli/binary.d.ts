/** The platform package for `platform` / `arch` (Node's names). */
export function platformPackage(platform?: string, arch?: string): string;

/** The absolute path of the `reactogenic` binary; throws when there is none. */
export function binaryPath(options?: {
  env?: NodeJS.ProcessEnv;
  platform?: string;
  arch?: string;
  resolve?: (id: string) => string;
}): string;
