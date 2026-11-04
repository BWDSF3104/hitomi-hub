import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join, resolve } from "node:path";

const harnessDir = dirname(fileURLToPath(import.meta.url));
export const root = join(harnessDir, "..");

export function projectPath(...parts) {
  return resolve(root, ...parts);
}

export function readProjectFile(...parts) {
  return readFileSync(projectPath(...parts), "utf8").replace(/\r\n/g, "\n");
}

export function bodyIn(html, openPattern, endPattern) {
  const start = html.indexOf(openPattern);
  const end = html.indexOf(endPattern);
  if (start < 0) throw new Error("body start not found: " + openPattern);
  if (end < start) throw new Error("body end not found: " + endPattern);
  return html.slice(start, end);
}

export function createChecker() {
  const failures = [];
  const passed = [];

  function check(cond, label) {
    if (cond) passed.push(label);
    else failures.push(label);
    return cond;
  }

  return { failures, passed, check };
}
