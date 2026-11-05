import { bodyIn, createChecker, readProjectFile } from "./harness_lib.mjs";

const viewerHtml = readProjectFile("app", "viewer.html");
const searchHtml = readProjectFile("app", "search.html");

const { failures, passed, check } = createChecker();

const viewerCheckBlock = bodyIn(viewerHtml, "const readListsDebounceMs = 250;", "\nasync function showPage() {");
const searchCheckBlock = bodyIn(searchHtml, "const readListsDebounceMs = 250;", "\nasync function runSearch() {");

check(
  viewerCheckBlock.includes("const readListsDebounceMs = 250;") && searchCheckBlock.includes("const readListsDebounceMs = 250;"),
  "read/lists debounce uses the 250ms delay constant"
);
check(searchCheckBlock.includes("function debounceLists() {"), "debounce helper exists for the lists display");

const viewerHelperBlock = viewerCheckBlock.slice(viewerCheckBlock.indexOf("function runBackgroundRead(showGen, openGen) {"));
const viewerShowPageBlock = bodyIn(viewerHtml, "async function showPage() {", "\nasync function openGallery(id, page, second) {");
check(
  viewerShowPageBlock.includes("currentPairEnd = doubleActive() && displayed.length > 1 ? displayed[1].page : null;") &&
  viewerShowPageBlock.includes("runBackgroundRead(showGen, openGen);") &&
  viewerShowPageBlock.indexOf("currentPairEnd = doubleActive() && displayed.length > 1 ? displayed[1].page : null;") < viewerShowPageBlock.indexOf("runBackgroundRead(showGen, openGen);"),
  "showPage records the displayed pair end before the debounced read helper call so the final double pair marks complete"
);
const searchExecBlock = searchCheckBlock.slice(searchCheckBlock.indexOf("let listsDebounceTimer = null;"));
const searchListsHelperBlock = searchExecBlock.slice(0, searchExecBlock.indexOf("async function showLists(searchGen) {"));

check(
  viewerHelperBlock.indexOf("clearTimeout(readListsDebounceTimer)") >= 0 &&
  viewerHelperBlock.indexOf("clearTimeout(readListsDebounceTimer)") < viewerHelperBlock.indexOf("setTimeout(() => {"),
  "read debounce clears the previous timer before rescheduling"
);
check(
  searchListsHelperBlock.indexOf("clearTimeout(listsDebounceTimer)") >= 0 &&
  searchListsHelperBlock.indexOf("clearTimeout(listsDebounceTimer)") < searchListsHelperBlock.indexOf("setTimeout(() => {"),
  "lists debounce clears the previous timer before rescheduling"
);
check(
  viewerHelperBlock.includes("if (showGen !== showGeneration || openGen !== openGeneration) return;") &&
  viewerHelperBlock.slice(viewerHelperBlock.indexOf("setTimeout(() => {")).includes("if (showGen !== showGeneration || openGen !== openGeneration) return;"),
  "read helper keeps the generation guard at entry and inside the debounced run"
);
check(viewerHelperBlock.includes("if (readKey === lastReadPostKey)"), "read helper skips the duplicate post for the same gallery and page");
check(viewerHelperBlock.includes("const lastShown = doubleActive() ? (Number(currentPairEnd) || current.page) : current.page;"), "read helper uses the pair end as the last shown page when double mode is active");
check(viewerHelperBlock.includes("const complete = lastPage > 0 && lastShown >= lastPage;"), "read helper computes complete from the last shown page (final single page or final double pair end)");

const searchListsBody = bodyIn(searchHtml, "async function showLists(searchGen) {", "\nasync function runSearch() {");
check(searchListsBody.includes("renderReadBadges();"), "showLists still refreshes card badges after the merge");
check(searchListsBody.includes("mergeReadState("), "showLists merges lists rows into the maps so card-observed state is kept");
check(!searchListsBody.includes("readStateByGallery.clear()") && !searchListsBody.includes("favoriteByGallery.clear()"), "showLists must not clear the card-observed state maps");
check(searchExecBlock.includes("badge.innerHTML = badgeInner("), "renderReadBadges renders through badgeInner so the inline reset buttons survive");
check(searchExecBlock.includes("const savedByGallery = new Map();"), "the saved state map is kept for the three state badges");
const cardBlock = searchExecBlock.slice(searchExecBlock.indexOf("async function processCard(id) {"), searchExecBlock.indexOf("async function worker() {"));
check(cardBlock.includes("completeCardState(card);"), "card response state completes the maps before the badge render");
const loadBody = bodyIn(viewerHtml, "async function loadFullImage(img, entry, signal, galleryId = current.id, loadShowGen = showGeneration, loadOpenGen = openGeneration, loadRender = renderGeneration, allowRecount = false) {", "\nfunction renderReader() {");
check(searchListsBody.includes("await debounceLists();"), "showLists waits for the debounce window before fetching");
check(searchListsBody.includes("if (!pending)") && searchListsBody.includes("listsFetchPending"), "showLists shares one in-flight lists fetch per window");
check(
  loadBody.includes('fetch(fullSrc, { cache: "no-store", signal })') &&
  loadBody.includes("URL.createObjectURL(blob)") &&
  loadBody.includes("img.src = objectUrl;") &&
  loadBody.includes("tn display fallback disabled") &&
  loadBody.includes('r.headers.get("x-cache")'),
  "full image display path unchanged: no-store fetch, objectURL display, x-cache judgement, TN display fallback disabled"
);

const counter = { readPosts: 0, listFetches: 0, lastReadBody: null };

const mockElements = {
  note: { textContent: "" },
  queries: { innerHTML: "" },
  favorites: { innerHTML: "" },
  unread: { innerHTML: "" },
  pagenum: { value: "" },
};

const documentMock = { getElementById: (id) => mockElements[id] || null };

const stubs = `
let showGeneration = 0;
let openGeneration = 0;
let readListsDebounceTimer = null;
let lastReadPostKey = "";
const readListsDebounceMs = 250;
let viewMode = "single";
let currentPairEnd = null;
function doubleActive() { return viewMode === "double"; }
let displaySource = "";
function highlightStrip() {}
function entrySource() { return "local"; }
function formatForEntry() { return "webp"; }
function setPrefetchStatus() {}
function ensureBackgroundPrefetch() {}
function refreshStrip() {}
function renderReader() {}
function renderPosition() {}
function renderViewState() {}
function setRoute() {}
function readerItems() {
  const pair = [{ page: current.page, entry: "e" }];
  if (viewMode === "double") pair.push({ page: current.page + 1, entry: "e" });
  return pair;
}
`;

const testScript = `
async function testRun() {
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  const viewerMove = (page) => {
    current.page = page;
    const showGen = ++showGeneration;
    runBackgroundRead(showGen, openGeneration);
  };

  const searchMove = (page) => {
    const searchGen = ++searchGeneration;
    showLists(searchGen);
  };

  // viewer page movements drive the viewer read post; search list calls drive the search lists fetch.
  for (let page = 2; page <= 6; page++) {
    viewerMove(page);
    searchMove(page);
    await sleep(20);
  }
  await sleep(800);

  check(counter.readPosts === 1, "consecutive page movements coalesce to one debounced read post");
  check(counter.listFetches === 1, "consecutive page movements coalesce to one debounced lists fetch");
  check(counter.lastReadBody && counter.lastReadBody.gallery_id === current.id && counter.lastReadBody.page === 6, "debounced post targets the latest gallery and page");
  check(counter.lastReadBody && counter.lastReadBody.complete === false, "non-final page movement posts with complete false");

  viewerMove(6);
  searchMove(6);
  await sleep(800);
  check(counter.readPosts === 1, "duplicate post for the same gallery and page is skipped");

  viewerMove(7);
  searchMove(7);
  await sleep(800);
  check(counter.readPosts === 2, "post runs again when the page changes");
  check(counter.lastReadBody && counter.lastReadBody.complete === true, "final page reach posts with complete true (reach alone marks fully_read)");

  const staleGen = showGeneration;
  showGeneration++;
  runBackgroundRead(staleGen, openGeneration);
  await sleep(800);
  check(counter.readPosts === 2, "stale show generation does not schedule a debounced post");

  const finalPair = { id: 102, page: 7, files: [1, 2, 3, 4, 5, 6, 7, 8], count: 8, sceneIndexes: [] };
  viewMode = "double";
  currentPairEnd = null;
  current = finalPair;
  showPage();
  await sleep(800);
  check(counter.readPosts === 3, "the final double pair display through showPage posts for the new gallery");
  check(counter.lastReadBody && counter.lastReadBody.page === 8 && counter.lastReadBody.complete === true, "showPage records the displayed pair end as the last shown page (final double pair marks complete true)");

  const gen = searchGeneration;
  const before = counter.listFetches;
  showLists(gen);
  showLists(gen);
  await sleep(800);
  check(counter.listFetches === before + 1, "concurrent showLists calls inside one window fetch once");
}
`;

const postStub = async (p, b) => {
  if (p === "/api/read") { counter.readPosts++; counter.lastReadBody = b; }
  return {};
};

const apiStub = async (p) => {
  if (p === "/api/lists") counter.listFetches++;
  return { queries: [], favorites: [], unread: [] };
};

const run = new Function(
  "current", "document", "post", "api", "check", "counter",
  stubs + "\n" + viewerHelperBlock + "\n" + viewerShowPageBlock + "\n" + searchExecBlock + "\n" + testScript + "\n return testRun();"
);

// count 7 makes page 7 the final page (scene_indexes sentinel or files length); pages 2-6 are not final.
const current0 = { id: 101, page: 1, files: [1, 2, 3, 4, 5, 6, 7], count: 7, sceneIndexes: [] };

await run(current0, documentMock, postStub, apiStub, check, counter);

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("mocked fetch harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);