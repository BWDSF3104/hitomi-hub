import { bodyIn, createChecker, readProjectFile } from "./harness_lib.mjs";

const html = readProjectFile("app", "viewer.html");
const workerBody = bodyIn(html, "async function worker() {", "\n  for (let i = 0; i < PREFETCH_CONCURRENCY; i++) worker();");

const { failures, passed, check } = createChecker();

const shiftAt = workerBody.indexOf("const item = prefetchQueue.shift();");
const guardAt = workerBody.indexOf("if (!item) return;");
const tryAt = workerBody.indexOf("try {");

check(shiftAt >= 0, "worker keeps prefetchQueue.shift() after the await window");
check(guardAt >= 0 && guardAt === workerBody.lastIndexOf("if (!item) return;"), "guard anchor: single if (!item) return; in worker");
check(guardAt > shiftAt && workerBody.slice(shiftAt + "const item = prefetchQueue.shift();".length, guardAt).trim() === "" && guardAt < tryAt, "guard position: immediately after shift and before try");
check(workerBody.includes("} catch (err) {") && workerBody.includes("updatePrefetchCounts(\"error\", item.entry, item.galleryId);"), "worker catch keeps the canonical error handling call");

const unguardedBody = workerBody.replace("if (!item) return;", "");
check(unguardedBody !== workerBody && !unguardedBody.includes("if (!item) return;"), "unguarded variant buildable from worker body for race reproduction");

async function runRace(body, concurrency, items) {
  const processed = [];
  const counts = { error: 0, cache: 0 };
  const exceptions = [];
  const queue = items.slice();
  const signal = { aborted: false };
  const currentPriorityDone = Promise.resolve();
  async function prefetchPageImage(entry) {
    processed.push(entry === undefined ? "<undefined>" : entry);
    await Promise.resolve();
    return "cache";
  }
  function updatePrefetchCounts(status, entry) {
    counts[status] = (counts[status] || 0) + 1;
    if (entry === undefined) counts["undefinedEntry"] = (counts["undefinedEntry"] || 0) + 1;
  }
  const worker = new Function("prefetchQueue", "currentPriorityDone", "signal", "prefetchPageImage", "updatePrefetchCounts", body + "\nreturn worker;")(queue, currentPriorityDone, signal, prefetchPageImage, updatePrefetchCounts);
  let normalEnds = 0;
  const promises = [];
  for (let i = 0; i < concurrency; i++) {
    promises.push(worker().then(() => { normalEnds++; }, (err) => exceptions.push(err.message)).catch(() => undefined));
  }
  await Promise.all(promises);
  return { processed, counts, exceptions, queueRemaining: queue.length, normalEnds };
}

async function runErrorPath(body, concurrency) {
  const processed = [];
  const counts = { error: 0, cache: 0 };
  const exceptions = [];
  const queue = [{ entry: "e1", galleryId: "g1" }, { entry: "e2", galleryId: "g1" }];
  const signal = { aborted: false };
  const currentPriorityDone = Promise.resolve();
  async function prefetchPageImage(entry) {
    processed.push(entry === undefined ? "<undefined>" : entry);
    if (entry === "e1") throw new Error("fetch failed (defined item)");
    await Promise.resolve();
    return "cache";
  }
  function updatePrefetchCounts(status, entry) {
    counts[status] = (counts[status] || 0) + 1;
    if (entry === undefined) counts["undefinedEntry"] = (counts["undefinedEntry"] || 0) + 1;
  }
  const worker = new Function("prefetchQueue", "currentPriorityDone", "signal", "prefetchPageImage", "updatePrefetchCounts", body + "\nreturn worker;")(queue, currentPriorityDone, signal, prefetchPageImage, updatePrefetchCounts);
  let normalEnds = 0;
  const promises = [];
  for (let i = 0; i < concurrency; i++) {
    promises.push(worker().then(() => { normalEnds++; }, (err) => exceptions.push(err.message)));
  }
  await Promise.all(promises);
  return { processed, counts, exceptions, queueRemaining: queue.length, normalEnds };
}

// race window: worker passes the length check, awaits, then shift() returns undefined
// because another worker took the last item (microtask registration order is deterministic)
const raceUnguarded = await runRace(unguardedBody, 2, [{ entry: "e1", galleryId: "g1" }]);
const raceGuarded = await runRace(workerBody, 2, [{ entry: "e1", galleryId: "g1" }]);
const errPath = await runErrorPath(workerBody, 1);

check(
  raceUnguarded.exceptions.length === 1 && raceUnguarded.exceptions[0].includes("Cannot read properties of undefined (reading 'entry')") && raceUnguarded.counts.error === 0 && raceUnguarded.processed.length === 1 && raceUnguarded.queueRemaining === 0 && raceUnguarded.normalEnds === 1,
  "race window exercised deterministically: unguarded variant leaks 1 TypeError reading 'entry' before any count with worker normal 1 exception 1"
);
check(
  raceGuarded.exceptions.length === 0 && raceGuarded.normalEnds === 2 && raceGuarded.processed.length === 1 && raceGuarded.processed[0] === "e1" && raceGuarded.counts.cache === 1 && raceGuarded.queueRemaining === 0 && !raceGuarded.counts.undefinedEntry,
  "guard version race: no exception, both workers end normally, undefined never reaches prefetch or counts"
);
check(
  errPath.exceptions.length === 0 && errPath.counts.error === 1 && errPath.processed.length === 2 && errPath.queueRemaining === 0 && !errPath.counts.undefinedEntry && errPath.normalEnds === 1,
  "error path identical with guard: defined item error counted once and worker continues to normal end"
);

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("viewer prefetch race harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
