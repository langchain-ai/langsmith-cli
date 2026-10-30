// The introspection helper's own toolchain: the Node.js it runs on, and tsx,
// esbuild and es-module-lexer, installed next to it in the CLI's cache.
//
// A failure here is never the project's: `NodeVersionError` asks for a newer
// Node.js, and `ToolchainError` says the cached packages are broken, which the
// CLI repairs by installing them again.
import * as module from "node:module";

/** `module.register`, which the hooks need, arrived in Node.js 20.6. */
export const MINIMUM_NODE = "20.6.0";

export class NodeVersionError extends Error {}

export class ToolchainError extends Error {}

/** Fail unless this Node.js can run the helper. */
export function checkNode() {
  if (compareVersions(process.versions.node, MINIMUM_NODE) < 0) {
    throw new NodeVersionError(
      `the TypeScript introspection helper needs Node.js ${MINIMUM_NODE} or newer, ` +
        `but ${process.execPath} is ${process.versions.node}`,
    );
  }
}

/**
 * Load every package of the toolchain and run esbuild once, so a broken install
 * fails here rather than as an import error in the project's first module.
 */
export async function loadToolchain() {
  try {
    await import("tsx/esm/api");
    await import("tsx/cjs/api");
    await import("./lex.mjs");
    const esbuild = await import("esbuild");
    esbuild.transformSync("export const ok: boolean = true;", { loader: "ts" });
  } catch (error) {
    throw new ToolchainError(`the TypeScript toolchain does not load: ${errorText(error)}`);
  }
}

/**
 * Register the text-module hooks, then tsx, so tsx's loader asks the hooks for
 * each module's source.
 */
export async function registerLoaders() {
  try {
    const hooks = await import("./hooks.mjs");
    if (typeof module.registerHooks === "function") {
      module.registerHooks(hooks.syncHooks);
    } else {
      module.register(new URL("./hooks.mjs", import.meta.url));
    }
    (await import("tsx/cjs/api")).register();
    (await import("tsx/esm/api")).register();
  } catch (error) {
    throw new ToolchainError(`the TypeScript loaders do not register: ${errorText(error)}`);
  }
}

export function errorText(error) {
  if (error instanceof Error) return `${error.name}: ${error.message}`;
  return String(error);
}

function compareVersions(a, b) {
  const left = a.split(".").map(Number);
  const right = b.split(".").map(Number);
  for (let index = 0; index < 3; index++) {
    const difference = (left[index] ?? 0) - (right[index] ?? 0);
    if (difference !== 0) return difference;
  }
  return 0;
}
