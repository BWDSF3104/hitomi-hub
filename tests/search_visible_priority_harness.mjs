import { bodyIn, createChecker, readProjectFile } from "./harness_lib.mjs";

const searchHtml = readProjectFile("app", "search.html");

const { failures, passed, check } = createChecker();

const helperBlock = bodyIn(searchHtml, "function placeholderElement(results, id) {", "async function loadSearchCards(ids, searchGen = searchGeneration) {");
const loadBlock = bodyIn(searchHtml, "async function loadSearchCards(ids, searchGen = searchGeneration) {", "\nfunction debounceLists() {");
const restoreBlock = bodyIn(searchHtml, "function restoreSearchState() {", "\nfunction searchTotalPages(total) {");
const runBlock = bodyIn(searchHtml, "async function runSearch() {", "\nfunction applyPrefetchCardKnowledge(card, gen) {");

check(searchHtml.includes("const SEARCH_CARD_CONCURRENCY = 8;"), "worker concurrency constant is 8");
check(searchHtml.includes("const FIRST_VISIBLE_EAGER_COVER_COUNT = 8;"), "first visible cover eager count is 8");
check(searchHtml.includes("opts.highPriority ? \"loading='eager' fetchpriority='high'\" : \"loading='lazy'\""), "renderSearchCard high priority opts emit eager fetchpriority for visible covers");
check(loadBlock.includes("const isCurrentlyVisible = visibleIds.has(id) || distanceToVisibleArea(id, visibleIds, results, visibleRoot) === 0;") && loadBlock.includes("const highPriority = eagerCoverCount < FIRST_VISIBLE_EAGER_COVER_COUNT && isCurrentlyVisible;"), "loadSearchCards uses first visible display-area cards for high priority covers");
check(loadBlock.includes("renderSearchCard(card, { highPriority })") && loadBlock.includes("loadSearchCardCover(card, searchGen, highPriority)"), "loadSearchCards passes highPriority to renderSearchCard and loadSearchCardCover");
check(helperBlock.includes("if (visibleIds.has(id)) return id;"), "takeNextSearchCard picks the current display-area card first");
check(helperBlock.includes("function distanceToVisibleArea(id, visibleIds, results, visibleRoot) {"), "candidates near the display area come before the rest of the queue");
check(loadBlock.includes("new IntersectionObserver(") && loadBlock.includes("observer.observe(el)"), "visibility comes from IntersectionObserver on the placeholder li");
check(loadBlock.includes("{ root: visibleRoot }"), "observer root is the results scroll container");
check(loadBlock.includes("const id = takeNextSearchCard(pending, visibleIds, results, visibleRoot);"), "worker pop uses priority selection takeNextSearchCard");
check(loadBlock.includes("const pendingSet = new Set(pending);") && loadBlock.includes("if (!pendingSet.has(id)) continue;") && loadBlock.includes("pendingSet.delete(id);"), "observer membership uses pendingSet kept in sync with the pending array");
check(!loadBlock.includes("queue.shift()"), "FIFO shift pop replaced by takeNextSearchCard");
check(!helperBlock.includes("setInterval(") && !loadBlock.includes("setInterval(") && !loadBlock.includes("requestAnimationFrame"), "no polling loop for the visibility judgement");
check(loadBlock.includes("if (searchGen !== searchGeneration) return;") && loadBlock.includes("const li = results.querySelector(\"[data-search-id='\" + id + \"']\");"), "search generation guard and card selector kept");
check(loadBlock.includes("const cardStart = performance.now();") && loadBlock.includes("recordPerfCache(card._x_cache);") && loadBlock.includes("renderPerfStatus();"), "per-card perf timing kept");
check(loadBlock.includes("const workerCount = Math.min(SEARCH_CARD_CONCURRENCY, ids.length);") && loadBlock.includes("await Promise.all(workers);"), "bounded workers kept");
check(!restoreBlock.includes("loadSearchCards(") && runBlock.includes("await loadSearchCards(ids, searchGen);"), "page-load card fetch stays single-run because restoreSearchState renders only and runSearch owns the card run");

const stubs = `
let searchGeneration = 1;
const SEARCH_CARD_CONCURRENCY = 1;
let order = [];
let hook = null;
let currentResults = null;
let currentVisible = new Set();
const ioHolder = {};
const document = { getElementById: (id) => (id === "search-status" ? { textContent: "" } : currentResults) };
class IntersectionObserver {
  constructor(cb, opts) { ioHolder.observer = this; this.cb = cb; }
  observe(el) { this.cb([{ target: el, isIntersecting: currentVisible.has(el.dataset.searchId) }], this); }
  disconnect() {}
}
async function api(p) {
  const id = Number(p.slice(p.lastIndexOf("/") + 1));
  order.push(id);
  if (hook) await hook(id);
  return { _x_cache: "cache" };
}
function setStatus() {}
function renderSearchCard(r) { return ""; }
async function loadSearchCardCover() {}
function recordPerfCache() {}
function renderPerfStatus() {}
function escapeHTML(s) { return String(s); }
const perfStatus = { cardsLoaded: 0, cardTotalMs: 0, cacheDistribution: {} };
`;

const testScript = `
async function runAll() {
  const rootRect = { top: 0, bottom: 300, left: 0, right: 400 };
  function makeResults(tops) {
    const elMap = new Map();
    for (const [id, top] of tops) {
      elMap.set(String(id), {
        dataset: { searchId: String(id) },
        getBoundingClientRect: () => ({ top, bottom: top + 130 }),
        outerHTML: "",
        innerHTML: "",
      });
    }
    return {
      elMap,
      querySelector: (sel) => elMap.get(String(sel.match(/data-search-id='(\\d+)'/)[1])),
      parentElement: { getBoundingClientRect: () => rootRect },
    };
  }
  async function runScenario(ids, tops, initialVisible, scenarioHook, searchGen) {
    currentResults = makeResults(tops);
    currentVisible = new Set(initialVisible.map(String));
    order = [];
    hook = scenarioHook;
    await loadSearchCards(ids, searchGen === undefined ? searchGeneration : searchGen);
    return order;
  }

  // scenario a: ids-order-earlier cards outside the display area must lose to display-area cards
  const topsA = [["101", 310], ["102", 320], ["103", 20], ["104", 340], ["105", 150], ["106", 360]];
  const orderA = await runScenario([101, 102, 103, 104, 105, 106], topsA, ["103", "105"], null);
  check(orderA[0] === 103 && orderA[1] === 105, "display-area cards are processed before ids-order-earlier cards outside the display area");
  check(orderA.indexOf(103) < orderA.indexOf(101) && orderA.indexOf(103) < orderA.indexOf(102), "visible card 103 beats ids-order-earlier off-area cards 101 and 102");
  check(orderA.length === 6, "one request per card keeps the total request count unchanged");

  // scenario b: a card that enters the display area during processing is promoted at the next pop
  const topsB = [["101", 310], ["102", 320], ["103", 330], ["104", 340], ["105", 350]];
  const orderB = await runScenario([101, 102, 103, 104, 105], topsB, [], async (id) => {
    if (id === 101) ioHolder.observer.cb([{ target: currentResults.elMap.get("103"), isIntersecting: true }], ioHolder.observer);
  });
  check(orderB[0] === 101, "before any card is visible the ids-order pop keeps working");
  check(orderB[1] === 103, "card that enters the display area during processing is promoted ahead of the remaining queue");
  check(orderB.indexOf(103) < orderB.indexOf(102), "the promoted card jumps ahead of the remaining ids-order queue");
  check(orderB.length === 5, "promotion adds no extra request");

  // scenario c: stale search generation is discarded
  searchGeneration = 2;
  const orderC = await runScenario([101, 102], [["101", 20], ["102", 40]], ["101"], null, 1);
  check(orderC.length === 0, "stale search generation is discarded by the generation guard");
}
`;

const run = new Function(
  "check",
  "performance",
  stubs + "\n" + helperBlock + "\n" + loadBlock + "\n" + testScript + "\n return runAll();"
);

await run(check, { now: () => 0 });

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("search visible priority harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
