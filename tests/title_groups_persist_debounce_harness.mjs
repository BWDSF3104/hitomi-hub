import { bodyIn, createChecker, readProjectFile } from "./harness_lib.mjs";

const searchHtml = readProjectFile("app", "search.html");

const { failures, passed, check } = createChecker();

const helperBlock = bodyIn(searchHtml, "function normalizeTitleKey(r) {", "function sameTitleJapaneseAnchor(r) {");
const renderBody = bodyIn(searchHtml, "function renderSearchCard(r, opts = {}) {", "async function loadSearchCardCover(");
const manualBlock = bodyIn(searchHtml, "const manualSearchInFlightKeys = new Set();", "function badgeFlag(label, className) {");
const schedulerBlock = bodyIn(searchHtml, "const PREFETCH_IDLE_MS = 800;", "document.getElementById(\"search\").addEventListener");

check(
  helperBlock.includes("function saveTitleGroups() {") && helperBlock.includes("function persistTitleGroups() {") &&
  helperBlock.includes("Object.fromEntries(titleGroups)") && helperBlock.includes("localStorage.setItem(TITLE_GROUPS_STORAGE_KEY"),
  "titleGroups persist keeps the full map payload through Object.fromEntries and localStorage.setItem(TITLE_GROUPS_STORAGE_KEY"
);
check(
  helperBlock.includes("const titleGroupsPersistDebounceMs = 250;") &&
  helperBlock.includes("function flushTitleGroups() {") &&
  helperBlock.indexOf("clearTimeout(titleGroupsPersistTimer)") >= 0 &&
  helperBlock.indexOf("clearTimeout(titleGroupsPersistTimer)") < helperBlock.indexOf("setTimeout(flushTitleGroups,"),
  "debounce clears the previous timer before rescheduling (existing debounce helper style)"
);
check(
  searchHtml.includes("const readListsDebounceMs = 250;") && searchHtml.includes("}, readListsDebounceMs);") &&
  helperBlock.includes("setTimeout(flushTitleGroups, titleGroupsPersistDebounceMs);") &&
  !helperBlock.includes("listsDebounceTimer") && !helperBlock.includes("listsWaiters"),
  "readListsDebounceMs canonical meaning untouched: 250 for /api/lists only and the titleGroups persist debounce uses its own constant"
);
check(
  helperBlock.includes("function setSearchTitleGroup(r) {") &&
  helperBlock.includes("if (!existing || existing.language !== \"japanese\" || r.language === \"japanese\")") &&
  helperBlock.includes("saveTitleGroups();") &&
  renderBody.includes("setSearchTitleGroup(r);") && !manualBlock.includes("localStorage") && !schedulerBlock.includes("localStorage"),
  "group decision condition and call points unchanged: condition intact, save called only inside the condition, block boundaries keep manual and scheduler storage-free"
);

const stats = { writes: 0, schedules: 0, lastPayload: null };

const storageMock = {
  rawStore: new Map(),
  getItem(k) { return this.rawStore.has(k) ? this.rawStore.get(k) : null; },
  setItem(k, v) { stats.writes++; stats.lastPayload = v; },
};

const clock = { t: 1000 };
const pendingTimers = [];
let timerSeq = 0;
function fakeSetTimeout(cb, ms) { stats.schedules++; pendingTimers.push({ id: ++timerSeq, cb, at: clock.t + ms }); return timerSeq; }
function fakeClearTimeout(id) { const i = pendingTimers.findIndex((t) => t.id === id); if (i >= 0) pendingTimers.splice(i, 1); }
function fireTimers() {
  const due = pendingTimers.splice(0).sort((a, b) => a.at - b.at);
  for (const t of due) { clock.t = t.at; t.cb(); }
}

const stubs = `
const titleGroups = new Map();
const TITLE_GROUPS_STORAGE_KEY = "hitomi_hub_title_groups";
function reset() {
  titleGroups.clear();
  localStorage.rawStore.clear();
  pendingTimers.length = 0;
  stats.writes = 0;
  stats.schedules = 0;
  stats.lastPayload = null;
  clock.t = 1000;
  lastTitleGroupsPersisted = null;
  titleGroupsPersistTimer = null;
}
`;

const testScript = `
async function runAll() {
  const card = (id, language, title) => ({ id, language, title });

  // scenario a: cold page 75 condition-matching calls coalesce to one write at the window end
  reset();
  for (let i = 1; i <= 75; i++) setSearchTitleGroup(card(1000 + i, "english", "cold title " + i));
  check(stats.writes === 0, "cold page writes nothing before the window end");
  check(pendingTimers.length === 1, "75 schedules collapse to one pending timer (rescheduling clears the previous timer)");
  check(stats.schedules === 75, "cold page: all 75 condition-matching calls schedule the debounced window");
  fireTimers();
  check(stats.writes === 1, "cold page window end flushes exactly one write");
  check(Object.keys(JSON.parse(stats.lastPayload)).length === 75, "flushed payload is the full map with all 75 entries");

  // scenario b: warm round trip re-render on unchanged content is equality skipped
  reset();
  for (let i = 1; i <= 25; i++) setSearchTitleGroup(card(2000 + i, "english", "warm title " + i));
  fireTimers();
  check(stats.writes === 1, "warm page first flush writes once against the empty baseline");
  stats.writes = 0; stats.schedules = 0;
  for (let i = 1; i <= 25; i++) setSearchTitleGroup(card(2000 + i, "english", "warm title " + i));
  check(stats.schedules === 25, "warm round trip still schedules 25 because the call path is unchanged");
  fireTimers();
  check(stats.writes === 0, "warm round trip with unchanged content is equality skipped at flush (0 writes)");

  // scenario c: group decision condition unchanged (mixed warm page: 15 updates, 10 skip)
  reset();
  for (let i = 1; i <= 10; i++) titleGroups.set("grp j" + i, { bestId: 3000 + i, language: "japanese", name: "grp j" + i });
  for (let i = 1; i <= 5; i++) titleGroups.set("grp e" + i, { bestId: 3100 + i, language: "english", name: "grp e" + i });
  for (let i = 1; i <= 10; i++) setSearchTitleGroup(card(4000 + i, "english", "grp j" + i)); // skip class
  for (let i = 1; i <= 5; i++) setSearchTitleGroup(card(4100 + i, "english", "grp e" + i)); // existing non-Japanese overwrite
  for (let i = 1; i <= 5; i++) setSearchTitleGroup(card(4200 + i, "english", "grp new" + i)); // new key
  for (let i = 1; i <= 5; i++) setSearchTitleGroup(card(3000 + i, "japanese", "grp j" + i)); // Japanese card
  check(stats.schedules === 15 && stats.writes === 0, "mixed warm page schedules 15 and skips 10 before the window end (condition classes preserved)");
  fireTimers();
  check(stats.writes === 1 && titleGroups.size === 20, "mixed warm page flushes one write for the 15 content changes");
  check(titleGroups.get("grp j1").bestId === 3001 && titleGroups.get("grp e1").bestId === 4101 && titleGroups.get("grp new1").bestId === 4201,
    "group decision order semantics intact: Japanese card wins, existing non-Japanese overwritten, new key added, skip class untouched");

  // scenario d: load seeds the equality baseline from stored raw
  reset();
  const seed = {};
  for (let i = 1; i <= 10; i++) seed["load t " + i] = { bestId: 5000 + i, language: "english", name: "load t " + i };
  const raw = JSON.stringify(seed);
  localStorage.rawStore.set(TITLE_GROUPS_STORAGE_KEY, raw);
  loadTitleGroups();
  check(lastTitleGroupsPersisted === raw, "load seeds the equality baseline from the stored raw string");
  for (let i = 1; i <= 10; i++) setSearchTitleGroup(card(5000 + i, "english", "load t " + i));
  fireTimers();
  check(stats.writes === 0, "no-change window after load writes nothing (equality skip against the loaded raw)");
  setSearchTitleGroup(card(6000, "japanese", "load t 1"));
  fireTimers();
  check(stats.writes === 1 && JSON.parse(stats.lastPayload)["load t 1"].bestId === 6000, "changed entry after load writes once with the full updated payload");

  // scenario e: re-measured improvement at n=10,000 warm map (old per-call sync vs debounced single flush)
  reset();
  for (let i = 1; i <= 10000; i++) titleGroups.set("m" + i, { bestId: i, language: "english", name: "m" + i });
  let syncCumMs = 0;
  for (let i = 1; i <= 75; i++) {
    const t0 = performance.now();
    localStorage.setItem(TITLE_GROUPS_STORAGE_KEY, JSON.stringify(Object.fromEntries(titleGroups)));
    syncCumMs += performance.now() - t0;
  }
  check(stats.writes === 75, "sync baseline writes 75 per page at n=10,000 (old behavior)");
  stats.writes = 0;
  titleGroups.set("m1", { bestId: 99999, language: "japanese", name: "m1" });
  for (let i = 1; i <= 75; i++) setSearchTitleGroup(card(7000 + i, "english", "m" + ((i % 10000) + 1)));
  const flushStart = performance.now();
  fireTimers();
  const flushMs = performance.now() - flushStart;
  check(stats.writes === 1 && syncCumMs > 300 && flushMs < 100, "re-measured n=10,000 warm page: debounced single flush (<100 ms) vs sync 75 writes cumulative (>300 ms)");
  console.log("measure n=10000: sync cumulative=" + syncCumMs.toFixed(1) + " ms (75 writes) -> debounced flush=" + flushMs.toFixed(1) + " ms (1 write)");

  return { syncCumMs, flushMs };
}
`;

const run = new Function(
  "check", "localStorage", "stats", "performance", "pendingTimers", "fireTimers", "clock", "setTimeout", "clearTimeout",
  stubs + "\n" + helperBlock + "\n" + testScript + "\n return runAll();"
);

await run(check, storageMock, stats, performance, pendingTimers, fireTimers, clock, fakeSetTimeout, fakeClearTimeout);

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("titleGroups persist debounce harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
