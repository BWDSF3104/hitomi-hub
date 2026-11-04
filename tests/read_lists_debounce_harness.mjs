import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const viewerHtml = readFileSync(join(root, "app", "viewer.html"), "utf8").replace(/\r\n/g, "\n");
const searchHtml = readFileSync(join(root, "app", "search.html"), "utf8").replace(/\r\n/g, "\n");

function bodyIn(html, openPattern, endPattern) {
  const start = html.indexOf(openPattern);
  const end = html.indexOf(endPattern);
  if (start < 0) throw new Error("body start not found: " + openPattern);
  if (end < start) throw new Error("body end not found: " + endPattern);
  return html.slice(start, end);
}

const failures = [];
const passed = [];

function check(cond, label) {
  if (cond) passed.push(label);
  else failures.push(label);
  return cond;
}

const viewerCheckBlock = bodyIn(viewerHtml, "const readListsDebounceMs = 250;", "\nasync function showPage() {");
const searchCheckBlock = bodyIn(searchHtml, "const readListsDebounceMs = 250;", "\nasync function runSearch() {");

check(
  viewerCheckBlock.includes("const readListsDebounceMs = 250;") && searchCheckBlock.includes("const readListsDebounceMs = 250;"),
  "read/lists debounce uses the 250ms delay constant"
);
check(searchCheckBlock.includes("function debounceLists() {"), "debounce helper exists for the lists display");

const viewerHelperBlock = viewerCheckBlock.slice(viewerCheckBlock.indexOf("function runBackgroundRead(showGen, openGen) {"));
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

const searchListsBody = bodyIn(searchHtml, "async function showLists(searchGen) {", "\nasync function runSearch() {");
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
};

const documentMock = { getElementById: (id) => mockElements[id] || null };

const stubs = `
let showGeneration = 0;
let openGeneration = 0;
let readListsDebounceTimer = null;
let lastReadPostKey = "";
const readListsDebounceMs = 250;
function highlightStrip() {}
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

  viewerMove(6);
  searchMove(6);
  await sleep(800);
  check(counter.readPosts === 1, "duplicate post for the same gallery and page is skipped");

  viewerMove(7);
  searchMove(7);
  await sleep(800);
  check(counter.readPosts === 2, "post runs again when the page changes");

  const staleGen = showGeneration;
  showGeneration++;
  runBackgroundRead(staleGen, openGeneration);
  await sleep(800);
  check(counter.readPosts === 2, "stale show generation does not schedule a debounced post");

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
  stubs + "\n" + viewerHelperBlock + "\n" + searchExecBlock + "\n" + testScript + "\n return testRun();"
);

const current0 = { id: 101, page: 1, files: [] };

await run(current0, documentMock, postStub, apiStub, check, counter);

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("mocked fetch harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);