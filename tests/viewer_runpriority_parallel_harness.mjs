import { bodyIn, createChecker, readProjectFile } from "./harness_lib.mjs";

const html = readProjectFile("app", "viewer.html");
const runBlock = bodyIn(html, "async function runPriority(displayed, signal) {", "\nfunction resetBackgroundPrefetch(displayed) {");

const { failures, passed, check } = createChecker();

const CALL = "loadFullImage(img, item.entry, signal, priorityGalleryId, priorityShowGen, priorityOpenGen, priorityRenderGen)";
const GUARD = "if (priorityShowGen !== showGeneration || priorityOpenGen !== openGeneration || priorityRenderGen !== renderGeneration || priorityGalleryId !== current.id) return;";

check(runBlock.includes("for (const item of displayed) {"), "dispatch order stays array order (iterate display array without sorting)");
check(runBlock.includes("if (img) dispatches.push(" + CALL + ");"), "display pair is dispatched in parallel (promise created per item, no await at dispatch)");
check(runBlock.includes("await Promise.all(dispatches);"), "runPriority waits the dispatch set instead of serial await per item");
check(!runBlock.includes("if (img) await " + CALL + ";"), "no serial await remains inside runPriority");
check(runBlock.includes("if (signal.aborted) return;"), "abort check kept at per-item dispatch start");
check(runBlock.includes(GUARD), "generation guard kept at per-item dispatch start");
check(html.includes("async function runPriority(displayed, signal) {") && html.includes("await currentPriorityDone;"), "viewer_test.go pinned substrings unchanged (runPriority header and await currentPriorityDone)");
check(html.includes("if (spreadDirection === \"rtl\" && items.length > 1) parts.reverse();"), "render display order (array order plus RTL reversal) unchanged");

function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }
function median(values) { const s = [...values].sort((a, b) => a - b); return s[Math.floor(s.length / 2)]; }

const IMAGE_LANE_INTERVAL_MS = 60;

function createGate() {
  let last = 0;
  return async function reserve() {
    const now = Date.now();
    let slot = now;
    if (now < last + IMAGE_LANE_INTERVAL_MS) slot = last + IMAGE_LANE_INTERVAL_MS;
    last = slot;
    const gateWait = slot - now;
    if (gateWait > 0) await sleep(gateWait);
    return { gateWait, dispatchAt: Date.now() };
  };
}

const serialReference = runBlock
  .replace("dispatches.push(" + CALL + ");", "await " + CALL + ";")
  .replace("await Promise.all(dispatches);", "");
check(serialReference !== runBlock && serialReference.includes("await " + CALL + ";") && !serialReference.includes("Promise.all"), "serial reference variant buildable from the same body for the before/after comparison");

async function runTrial(body, displayed) {
  const trace = [];
  const gate = createGate();
  async function loadFullImage(img, entry, signal) {
    const arrival = Date.now();
    const g = await gate();
    trace.push({ page: entry.page, arrival, gateWait: g.gateWait, dispatchAt: g.dispatchAt });
    await sleep(entry.networkMs);
    trace.push({ page: entry.page, completedAt: Date.now() });
  }
  const mockDocument = { getElementById: () => ({ querySelector: (sel) => { const m = sel.match(/data-page='(\d+)'/); return m ? { page: Number(m[1]) } : null; } }) };
  const fn = new Function("displayed", "signal", "loadFullImage", "document", "showGeneration", "openGeneration", "renderGeneration", "current", body + "\nreturn runPriority(displayed, signal);");
  const start = Date.now();
  await fn(displayed, { aborted: false }, loadFullImage, mockDocument, 1, 1, 1, { id: "g1", page: 1 });
  const events = trace.filter((e) => e.dispatchAt !== undefined);
  return { wall: Date.now() - start, events, trace };
}

function pair(displayed) {
  return displayed.map((item) => ({ page: item.page, entry: { page: item.page, networkMs: item.networkMs } }));
}

const NETWORK_MS = 679;
const TRIALS = 5;
const serialWalls = [];
const parallelWalls = [];
const serialGateWait2 = [];
const parallelGateWait2 = [];
const parallelDispatchGap = [];

for (let t = 0; t < TRIALS; t++) {
  const serial = await runTrial(serialReference, pair([{ page: 1, networkMs: NETWORK_MS }, { page: 2, networkMs: NETWORK_MS }]));
  const parallel = await runTrial(runBlock, pair([{ page: 1, networkMs: NETWORK_MS }, { page: 2, networkMs: NETWORK_MS }]));
  serialWalls.push(serial.wall);
  parallelWalls.push(parallel.wall);
  serialGateWait2.push(serial.events[1].gateWait);
  parallelGateWait2.push(parallel.events[1].gateWait);
  parallelDispatchGap.push(parallel.events[1].dispatchAt - parallel.events[0].dispatchAt);
}

const serialMedian = median(serialWalls);
const parallelMedian = median(parallelWalls);
console.log("SERIAL reference median wall: " + serialMedian + " ms");
console.log("PARALLEL (current code) median wall: " + parallelMedian + " ms");
console.log("SERIAL second gate wait median: " + median(serialGateWait2) + " ms (gate idle, wasted wait is network serialization)");
console.log("PARALLEL second gate wait median: " + median(parallelGateWait2) + " ms");
console.log("PARALLEL dispatch gap median: " + median(parallelDispatchGap) + " ms (image lane interval 60 ms respected)");

check(Math.abs(serialMedian - 2 * NETWORK_MS) < 60, "serial reference still costs 2x network wall (before baseline reproduced)");
check(Math.abs(parallelMedian - (NETWORK_MS + IMAGE_LANE_INTERVAL_MS)) < 60, "current code costs network plus one lane interval (cold pair wall halved)");
check(parallelMedian <= serialMedian * 0.6, "measured improvement: parallel wall is about half of serial wall");
check(median(parallelDispatchGap) >= IMAGE_LANE_INTERVAL_MS && median(parallelDispatchGap) <= IMAGE_LANE_INTERVAL_MS + 10, "parallel dispatch keeps the 60 ms image lane dispatch interval");
check(parallelDispatchGap.every((g) => g >= IMAGE_LANE_INTERVAL_MS), "every parallel trial respects the image lane interval (no faster dispatch)");

const orderTrial = await runTrial(runBlock, pair([{ page: 1, networkMs: 957 }, { page: 2, networkMs: 235 }]));
const dispatchOrder = orderTrial.events.map((e) => e.page);
const completionOrder = orderTrial.trace.filter((e) => e.completedAt !== undefined).map((e) => e.page);
console.log("ASYMMETRIC parallel: dispatch order " + dispatchOrder.join(",") + ", completion order " + completionOrder.join(","));
check(dispatchOrder.join(",") === "1,2", "dispatch order remains array order when per-file network times differ");
check(completionOrder.join(",") === "2,1", "completion order can follow arrival order (agreed side-effect of parallel dispatch)");

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("viewer runPriority parallel harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
