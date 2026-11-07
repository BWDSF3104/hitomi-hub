import { bodyIn, createChecker, readProjectFile } from "./harness_lib.mjs";

const searchHtml = readProjectFile("app", "search.html");

const { failures, passed, check } = createChecker();

const helperBlock = bodyIn(searchHtml, "function placeholderElement(results, id) {", "async function loadSearchCards(ids, searchGen = searchGeneration) {");
const loadBlock = bodyIn(searchHtml, "async function loadSearchCards(ids, searchGen = searchGeneration) {", "\nfunction debounceLists() {");
const restoreBlock = bodyIn(searchHtml, "function restoreSearchState() {", "\nfunction searchTotalPages(total) {");
const runBlock = bodyIn(searchHtml, "async function runSearch() {", "\nfunction applyPrefetchCardKnowledge(card, gen) {");
const initBlock = bodyIn(searchHtml, "loadTitleGroups();", "updateJumpButton();");

check(searchHtml.includes("let searchGeneration = 0;"), "searchGeneration starts at 0 for the page load");
check(searchHtml.includes("const searchGen = ++searchGeneration;"), "runSearch advances the generation for its own card run");
check(initBlock.includes("restoreSearchState();") && initBlock.includes("runSearch();"), "page load runs restoreSearchState then runSearch");
check(restoreBlock.includes("renderSearchPageStatus();"), "restoreSearchState keeps rendering the saved page status");
check(loadBlock.includes("if (searchGen !== searchGeneration) return;"), "generation guard is kept in the card run");
check(!restoreBlock.includes("loadSearchCards("), "restoreSearchState renders placeholders only and does not call loadSearchCards");

const stubs = `
let searchGeneration = 0;
const SEARCH_CARD_CONCURRENCY = 8;
const FIRST_VISIBLE_EAGER_COVER_COUNT = 8;
let pendingHighlightId = null;
const perfStatus = { searchMs: 0, serverTiming: "", cardsLoaded: 0, cardTotalMs: 0, cacheDistribution: {} };
const requests = [];
const renderAtGen = [];
function el(name, value) { return { name, value, textContent: "", disabled: false, dataset: {}, classList: { toggle() {} }, addEventListener() {} }; }
const resultsEl = {
  elMap: new Map(),
  querySelector(sel) { const m = sel.match(/data-search-id='(\\d+)'/); return m ? resultsEl.elMap.get(m[1]) || null : null; },
  parentElement: { getBoundingClientRect: () => ({ top: 0, bottom: 300, left: 0, right: 400 }) },
};
Object.defineProperty(resultsEl, "innerHTML", { get: () => "", set: (html) => { resultsEl.elMap = new Map(); for (const m of String(html).matchAll(/data-search-id='(\\d+)'/g)) { resultsEl.elMap.set(m[1], { dataset: { searchId: m[1] }, innerHTML: "", outerHTML: "", getBoundingClientRect: () => ({ top: 0, bottom: 130, left: 0, right: 200 }) }); } } });
const els = { results: resultsEl, query: el("query", "furry"), lang: el("lang", "all"), page: el("page", "1") };
const document = { getElementById: (name) => els[name] || (els[name] = el(name, "")) };
function setStatus() {}
function placeholderCard(id) { return "<li data-search-id='" + id + "'></li>"; }
function renderSearchCard(r) { renderAtGen.push(searchGeneration); return "<li data-search-id='" + r.id + "'></li>"; }
async function loadSearchCardCover() {}
function completeCardState() {}
function recordPerfCache(v) { if (v) perfStatus.cacheDistribution[v] = (perfStatus.cacheDistribution[v] || 0) + 1; }
function renderPerfStatus() {}
function renderSearchPageStatus() {}
function escapeHTML(s) { return String(s ?? ""); }
function saveSearchState() {}
function loadSearchState() { return state; }
function parseSearchHash() { return null; }
function searchRoute(query, page) { return "#/q/" + encodeURIComponent(query) + "/page/" + page; }
async function showLists() {}
function loadTitleGroups() {}
function armPrefetchIdle() {}
function highlightCard() {}
function updateJumpButton() {}
function searchTotalPages(total) { return Math.max(1, Math.ceil(Number(total || 0) / 25)); }
function placeholderElement(results, id) { return results.elMap.get(String(id)) || null; }
function distanceToVisibleArea() { return Number.MAX_SAFE_INTEGER; }
function takeNextSearchCard(pending) { return pending[0]; }
class IntersectionObserver { constructor(cb) { this.cb = cb; } observe() {} disconnect() {} }
async function api(p) {
  requests.push({ path: p, genAtCall: searchGeneration });
  if (p.startsWith("/api/search?")) { await Promise.resolve(); return { total: state ? state.total : 25, page: state ? state.page : 1, ids: state ? state.ids : [201,202,203,204,205,206,207,208,209,210,211,212,213,214,215,216,217,218,219,220,221,222,223,224,225], _x_cache: "cache", _x_search_timing: "" }; }
  const id = Number(p.slice(p.lastIndexOf("/") + 1));
  await Promise.resolve();
  return { id, _x_cache: "cache", title: "x", language: "", type: "", count: 0, files: [] };
}
const history = { replaceState() {} };
const window = { location: { hash: "" } };
`;

async function runStartupScenario(state) {
  const run = new Function(
    "check",
    "performance",
    "state",
    stubs + "\n" + helperBlock + "\n" + restoreBlock + "\n" + loadBlock + "\n" + runBlock + "\n return (async () => { let error = null; try { " + initBlock + " } catch (err) { error = err.message; } await new Promise((r) => setTimeout(r, 0)); return { requests, renderAtGen, cardsLoaded: perfStatus.cardsLoaded, error }; })();"
  );
  return run(check, { now: () => 0 }, state);
}

function summarize(out) {
  const cardReqs = out.requests.filter((r) => r.path.startsWith("/api/search-card/"));
  const searchReqIndex = out.requests.findIndex((r) => r.path.startsWith("/api/search?"));
  const idsSeen = new Set();
  const duplicated = cardReqs.filter((r) => {
    if (idsSeen.has(r.path)) return true;
    idsSeen.add(r.path);
    return false;
  });
  return {
    cardRequests: cardReqs.length,
    duplicatedRequests: duplicated.length,
    cardRequestsBeforeSearch: cardReqs.filter((r) => searchReqIndex >= 0 && out.requests.indexOf(r) < searchReqIndex).length,
    renderAtGen: out.renderAtGen,
    cardsLoaded: out.cardsLoaded,
    error: out.error,
  };
}

// page load with saved state (the typical case that ran card fetch twice before the fix)
const withState = summarize(await runStartupScenario({ query: "furry", lang: "all", page: 1, ids: [101,102,103,104,105,106,107,108,109,110,111,112,113,114,115,116,117,118,119,120,121,122,123,124,125], total: 25 }));
check(!withState.error, "page load with saved state runs the startup sequence without error");
check(withState.cardRequests === 25, "page load with saved state fetches cards once per id (25 card requests, 1 run)");
check(withState.duplicatedRequests === 0, "page load with saved state fetches no card twice (no restore run)");
check(withState.cardRequestsBeforeSearch === 0, "card fetch follows the /api/search response on page load (no pre-response restore prefetch)");
check(withState.renderAtGen.every((g) => g === 1) && withState.cardsLoaded === 25, "page load with saved state still processes and displays the same 25 cards");

// page load without saved state (isolated origin first load)
const withoutState = summarize(await runStartupScenario(null));
check(!withoutState.error, "page load without saved state runs the startup sequence without error");
check(withoutState.cardRequests === 25, "page load without saved state fetches cards once per id (25 card requests, 1 run)");
check(withoutState.duplicatedRequests === 0, "page load without saved state has no restore run to duplicate");
check(withoutState.cardRequestsBeforeSearch === 0, "page load without saved state keeps card fetch after the search response");
check(withoutState.cardsLoaded === 25, "page load without saved state processes the same 25 cards");

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("R-12 startup single run harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
