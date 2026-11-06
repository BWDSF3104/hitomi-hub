import { bodyIn, createChecker, readProjectFile } from "./harness_lib.mjs";

const searchHtml = readProjectFile("app", "search.html");

const { failures, passed, check } = createChecker();

const schedulerBlock = bodyIn(searchHtml, "const PREFETCH_IDLE_MS = 800;", "document.getElementById(\"search\").addEventListener");
const patchHelperBlock = bodyIn(searchHtml, "function applyPrefetchCardKnowledge(card, gen) {", "const PREFETCH_IDLE_MS = 800;");
const runSearchBlock = bodyIn(searchHtml, "async function runSearch() {", "const PREFETCH_IDLE_MS = 800;");
const wiringBlock = searchHtml.slice(searchHtml.indexOf("document.getElementById(\"search\").addEventListener"));
const totalBlock = bodyIn(searchHtml, "function searchTotalPages(total) {", "function recordPerfCache(value) {");
const coverBlock = bodyIn(searchHtml, "function searchCardCoverSource(card) {", "async function loadSearchCardCover(");
const loadBlock = bodyIn(searchHtml, "async function loadSearchCards(ids, searchGen = searchGeneration) {", "\nfunction debounceLists() {");

check(
  schedulerBlock.includes("const PREFETCH_IDLE_MS = 800;") && schedulerBlock.includes("const PREFETCH_CONCURRENCY = 4;") && schedulerBlock.includes("const PREFETCH_COOLDOWN_MS = 300000;"),
  "scheduler keeps the fixed constants idle 800ms, concurrency 4, cooldown 5 minutes"
);
check(schedulerBlock.includes("const PREFETCH_AHEAD_PAGES = 2;") && schedulerBlock.includes("offset <= PREFETCH_AHEAD_PAGES"), "prefetch ahead page count is fixed at the p95 measured recommendation 2");
check(patchHelperBlock.includes("appendChild") && patchHelperBlock.includes("setSearchTitleGroup") && patchHelperBlock.includes("group.language") && patchHelperBlock.includes("Number(group.bestId) === Number(card.id)") && !patchHelperBlock.includes("innerHTML") && schedulerBlock.includes("applyPrefetchCardKnowledge(card, gen)"), "success patch helper is outside schedulerBlock and uses appendChild not innerHTML");
check(schedulerBlock.includes("\"priority=background\""), "prefetch requests carry the background marker as a query param");
check(schedulerBlock.includes("api(prefetchMarkerUrl("), "prefetch requests go through the existing api() helper with the marker");
check(searchHtml.includes("const api = (p) => fetch(p, { cache: \"no-store\" })") && searchHtml.includes("err.status = r.status;"), "api stays no-store and exposes the response status for the 429/5xx stop rule");
check(runSearchBlock.includes("searchCardsCompleted = false;") && runSearchBlock.includes("searchCardsCompleted = true;") && runSearchBlock.includes("armPrefetchIdle();"), "runSearch arms the idle timer after the current page cards complete");
check(wiringBlock.includes("document.getElementById(\"query\").addEventListener(\"input\", () => { interruptPrefetch(); });"), "query input interrupts the prefetch");
check(wiringBlock.includes("document.getElementById(\"search-next\").addEventListener(\"mouseenter\", () => { startPrefetchNow(); });") && wiringBlock.includes("document.getElementById(\"search-next\").addEventListener(\"focus\", () => { startPrefetchNow(); });"), "next button hover or focus starts the prefetch immediately");
check(wiringBlock.includes("if (e.target.closest(\"a\")) interruptPrefetch();"), "opening a gallery from a card interrupts the prefetch");
check(wiringBlock.includes("document.addEventListener(\"visibilitychange\", () => {") && wiringBlock.includes("if (document.hidden) interruptPrefetch();"), "a hidden tab interrupts the prefetch");
check(wiringBlock.includes("prefetchEligible = false;"), "saved-query startup does not trigger the idle prefetch");
check(wiringBlock.includes("prefetchEligible = true;"), "the search form re-enables the idle prefetch");
check(!schedulerBlock.includes("setInterval("), "no polling loop or resident periodic work for the prefetch");
check(!schedulerBlock.includes("setStatus(") && !schedulerBlock.includes("renderSearchCard(") && !schedulerBlock.includes("innerHTML"), "prefetch failures are not rendered on screen");
check(!schedulerBlock.includes("/api/image/"), "prefetch targets meta and TN covers only, never full images");
check(!schedulerBlock.includes("localStorage"), "cooldown is memory-only, never persisted");
check(schedulerBlock.includes("gen !== searchGeneration"), "prefetch is bound to the existing searchGeneration concept");
check(schedulerBlock.includes("Math.min(PREFETCH_CONCURRENCY, pending.length)") && schedulerBlock.includes("await Promise.all(workers)"), "prefetch runs bounded workers at concurrency 4");
check(searchHtml.includes("const SEARCH_CARD_CONCURRENCY = 8;") && loadBlock.includes("takeNextSearchCard"), "Phase 1 visible priority and foreground concurrency 8 are applied");
check(schedulerBlock.includes("prefetchInterrupted(gen)") && schedulerBlock.includes("if (onCard) onCard(card);"), "the interrupt condition is checked per completed request");

const stubs = `
let searchGeneration = 1;
let currentSearchTotal = 0;
let fakeNow = 1000;
let hook = null;
let active = 0;
let maxActive = 0;
let scenarioIds = [];
const requests = [];
const timers = [];
const inputs = { query: "female:yuri", lang: "all", page: "1" };
const resultsElement = { querySelector: () => null };
const document = {
  getElementById: (id) => id === "results" ? resultsElement : ({ value: inputs[id] || "" }),
};
const titleGroups = new Map();
let newTabMode = false;
function normalizeTitleKey(r) {
  return String(r.title || r.japanese_title || r.id || "").trim().toLowerCase();
}
function setSearchTitleGroup(r) {
  const key = normalizeTitleKey(r);
  if (!key || r.id === undefined) return;
  titleGroups.set(key, { bestId: r.id, language: r.language || "", name: r.japanese_title || r.title || String(r.id) });
}
function viewerLink(id) {
  return "/#/g/" + id;
}
const Date = { now: () => fakeNow };
const setTimeout = (cb, ms) => { timers.push({ cb, ms }); return timers.length; };
const clearTimeout = () => {};
async function api(p) {
  requests.push(p);
  active++; if (active > maxActive) maxActive = active;
  if (hook) { const r = hook(p); if (r) await r; }
  await Promise.resolve();
  active--;
  const path = p.split("?")[0];
  if (path === "/api/search") return { ids: scenarioIds, total: currentSearchTotal };
  if (path.startsWith("/api/search-card/")) {
    const id = Number(path.slice("/api/search-card/".length));
    return { id, thumbnail: "/api/thumb/" + "h" + id + "?format=webp&gallery=" + id + "&variant=bigtn" };
  }
  return {};
}
async function settle() { while (prefetchRunning) await Promise.resolve(); }
async function fireTimers() { const fired = timers.splice(0).map((t) => t.cb()); await Promise.all(fired); await settle(); }
function resetScenario(gen) {
  searchGeneration = gen;
  fakeNow = 1000;
  hook = null;
  active = 0;
  maxActive = 0;
  requests.length = 0;
  timers.length = 0;
  prefetchCooldownUntil = 0;
  prefetchInterruptFlag = false;
  prefetchEligible = true;
  searchCardsCompleted = false;
  prefetchIdleTimer = null;
  prefetchRunning = false;
}
`;

const testScript = `
async function runAll() {
  const fixedPages = PREFETCH_AHEAD_PAGES;
  const firstAheadPage = 2;
  const targetPage = 1 + fixedPages;

  // scenario a: current page completed plus 800ms idle starts the first-ahead-page prefetch
  resetScenario(1);
  currentSearchTotal = firstAheadPage * 25;
  scenarioIds = [201, 202, 203, 204];
  searchCardsCompleted = true;
  armPrefetchIdle();
  check(timers.length === 1, "idle wait uses the 800ms timer constant, not an immediate start");
  check(requests.length === 0, "no prefetch request before the idle window elapses");
  await fireTimers();
  const searchReq = requests[0];
  check(searchReq.startsWith("/api/search?query=female%3Ayuri&lang=all&page=" + firstAheadPage + "&limit=25"), "first prefetch request is the next page id list in the current runSearch query format");
  check(searchReq.includes("priority=background"), "the id list request carries the marker");
  const cardReqs = requests.filter((p) => p.split("?")[0].startsWith("/api/search-card/"));
  const coverReqs = requests.filter((p) => p.split("?")[0].startsWith("/api/thumb/"));
  check(cardReqs.length === 4 && coverReqs.length === 4, "one meta request per next-page id and one TN cover request per card");
  const firstCover = Math.min(...coverReqs.map((p) => requests.indexOf(p)));
  const lastCard = Math.max(...cardReqs.map((p) => requests.indexOf(p)));
  check(lastCard < firstCover, "covers are warmed after the meta phase");
  check(coverReqs.every((p) => p.includes("priority=background")), "TN cover warm uses the current cover URL format with the marker");
  check(requests.every((p) => p.includes("priority=background")), "every prefetch request carries the background marker");
  check(maxActive === 4 && maxActive <= 4, "prefetch runs exactly four workers in parallel");

  // scenario b1: a new search generation stops the prefetch from issuing the next request
  resetScenario(5);
  currentSearchTotal = 50;
  scenarioIds = [201, 202, 203];
  searchCardsCompleted = true;
  hook = async (p) => {
    if (p.split("?")[0].startsWith("/api/search-card/") && requests.length === 2) searchGeneration = 6;
  };
  await runPrefetch();
  check(requests.filter((p) => p.split("?")[0].startsWith("/api/search-card/")).length === 1, "new search generation stops the prefetch before the next request");
  check(!requests.some((p) => p.split("?")[0].startsWith("/api/thumb/")), "interrupted prefetch issues no cover requests after the stop");

  // scenario b2: query input (user starts thinking about the next search) stops the prefetch
  resetScenario(1);
  currentSearchTotal = 50;
  scenarioIds = [201, 202, 203];
  searchCardsCompleted = true;
  hook = async (p) => {
    if (requests.length === 2) interruptPrefetch();
  };
  await runPrefetch();
  check(requests.filter((p) => p.split("?")[0].startsWith("/api/search-card/")).length === 1, "query input interrupt stops the prefetch before the next request");
  check(requests.length === 2, "interrupted run stops at the request unit without re-issuing");

  // scenario c: 429 stops everything and the cooldown blocks resume
  resetScenario(1);
  currentSearchTotal = 50;
  scenarioIds = [201];
  searchCardsCompleted = true;
  hook = async (p) => {
    if (p.split("?")[0].startsWith("/api/search-card/")) {
      const err = new Error("429 " + p);
      err.status = 429;
      throw err;
    }
  };
  await runPrefetch();
  check(prefetchCooldownUntil === fakeNow + 300000, "429 sets a 5 minute cooldown in memory");
  check(requests.filter((p) => p.split("?")[0].startsWith("/api/search-card/")).length === 1, "the failed prefetch request is not retried from the UI side");
  const beforeCooldown = requests.length;
  armPrefetchIdle();
  await fireTimers();
  check(requests.length === beforeCooldown, "no prefetch resumes during the cooldown");
  fakeNow += 300001;
  armPrefetchIdle();
  await fireTimers();
  check(requests.length > beforeCooldown, "prefetch may resume after the cooldown expires");

  // scenario d: the target never exceeds the configured fixed ahead-page count
  resetScenario(1);
  currentSearchTotal = 25;
  scenarioIds = [201];
  searchCardsCompleted = true;
  armPrefetchIdle();
  await fireTimers();
  check(requests.length === 0, "prefetch does not start when there is no ahead page beyond the current page");

  currentSearchTotal = targetPage * 25;
  armPrefetchIdle();
  await fireTimers();
  const searchPages = requests.filter((p) => p.split("?")[0] === "/api/search").map((p) => Number(p.split("page=")[1].split("&")[0]));
  check(searchPages.every((page) => page > 1 && page <= targetPage) && [firstAheadPage, targetPage].every((page) => searchPages.includes(page)) && !searchPages.includes(targetPage + 1), "the prefetch targets configured fixed ahead pages only, never beyond them");
  check(searchPages.length === fixedPages, "one prefetch run covers at most the configured fixed ahead-page count");

  // scenario e: saved-query startup is not eligible
  resetScenario(1);
  currentSearchTotal = 50;
  scenarioIds = [201];
  searchCardsCompleted = true;
  prefetchEligible = false;
  armPrefetchIdle();
  await fireTimers();
  check(requests.length === 0, "saved-query startup does not trigger the idle prefetch");
}
`;

const run = new Function(
  "check",
  "totalBlockUnused",
  stubs + "\n" + totalBlock + "\n" + coverBlock + "\n" + patchHelperBlock + "\n" + schedulerBlock + "\n" + testScript + "\n return runAll();"
);

await run(check, null);

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("search prefetch harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
