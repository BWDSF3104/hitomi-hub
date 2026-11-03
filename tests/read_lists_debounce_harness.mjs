import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const html = readFileSync(join(root, "app", "ui.html"), "utf8");

function body(openPattern, endPattern) {
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

const debounceBlock = body("const readListsDebounceMs = 250;", "\nasync function showPage() {") + "\n" + body("async function showLists(showGen, openGen) {", "\nfunction renderStripLoading(id) {");

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
function escapeHTML(s) { return String(s); }
function targetAttr() { return ""; }
function highlightStrip() {}
`;

const testScript = `
async function testRun() {
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  check(debounceBlock.includes("const readListsDebounceMs = 250;"), "read/lists debounce uses the 250ms delay constant");
  check(debounceBlock.includes("function debounceLists() {"), "debounce helper exists for the lists display");
  const helperStart = debounceBlock.indexOf("function runBackgroundReadAndLists(showGen, openGen) {");
  const helperBlock = debounceBlock.slice(helperStart);
  const listsHelperBlock = debounceBlock.slice(0, helperStart);
  check(helperBlock.indexOf("clearTimeout(readListsDebounceTimer)") < helperBlock.indexOf("setTimeout(() => {"), "read debounce clears the previous timer before rescheduling");
  check(listsHelperBlock.indexOf("clearTimeout(listsDebounceTimer)") < listsHelperBlock.indexOf("setTimeout(() => {"), "lists debounce clears the previous timer before rescheduling");
  check(helperBlock.includes("if (showGen !== showGeneration || openGen !== openGeneration) return;") && helperBlock.slice(helperBlock.indexOf("setTimeout(() => {")).includes("if (showGen !== showGeneration || openGen !== openGeneration) return;"), "read helper keeps the generation guard at entry and inside the debounced run");
  check(helperBlock.includes("if (readKey === lastReadPostKey)"), "read helper skips the duplicate post for the same gallery and page");

  const showPageMove = (page) => {
    current.page = page;
    const showGen = ++showGeneration;
    runBackgroundReadAndLists(showGen, openGeneration);
  };

  // consecutive page movements inside one debounce window coalesce to a single read post and a single lists fetch
  for (let page = 2; page <= 6; page++) { showPageMove(page); await sleep(20); }
  await sleep(800);
  check(counter.readPosts === 1, "consecutive page movements coalesce to one debounced read post");
  check(counter.listFetches === 1, "consecutive page movements coalesce to one debounced lists fetch");
  check(counter.lastReadBody && counter.lastReadBody.gallery_id === current.id && counter.lastReadBody.page === 6, "debounced post targets the latest gallery and page");

  // re-showing the same gallery/page skips the duplicate post
  showPageMove(6);
  await sleep(800);
  check(counter.readPosts === 1, "duplicate post for the same gallery and page is skipped");

  // a page change posts again
  showPageMove(7);
  await sleep(800);
  check(counter.readPosts === 2, "post runs again when the page changes");

  // stale generation guard must prevent a debounced post
  const staleGen = showGeneration;
  showGeneration++;
  runBackgroundReadAndLists(staleGen, openGeneration);
  await sleep(800);
  check(counter.readPosts === 2, "stale show generation does not schedule a debounced post");

  // concurrent showLists calls inside one window share one fetch
  const listsStart = html.indexOf("async function showLists(showGen, openGen) {");
  const listsBody = html.slice(listsStart, html.indexOf("\\nfunction renderStripLoading(id) {"));
  check(listsBody.includes("await debounceLists();"), "showLists waits for the debounce window before fetching");
  check(listsBody.includes("if (!pending)") && listsBody.includes("listsFetchPending"), "showLists shares one in-flight lists fetch per window");

  const gen = showGeneration;
  const before = counter.listFetches;
  showLists(gen, openGeneration);
  showLists(gen, openGeneration);
  await sleep(800);
  check(counter.listFetches === before + 1, "concurrent showLists calls inside one window fetch once");

  const loadStart = html.indexOf("async function loadFullImage(img, entry, signal, galleryId = current.id, loadShowGen = showGeneration, loadOpenGen = openGeneration, loadRender = renderGeneration, allowRecount = false) {");
  const loadBody = html.slice(loadStart, html.indexOf("\\nfunction renderReader() {"));
  check(loadBody.includes('fetch(fullSrc, { cache: "no-store", signal })') && loadBody.includes("URL.createObjectURL(blob)") && loadBody.includes("img.src = objectUrl;") && loadBody.includes("tn display fallback disabled") && loadBody.includes('r.headers.get("x-cache")'), "full image display path unchanged: no-store fetch, objectURL display, x-cache judgement, TN display fallback disabled");
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
  "current", "document", "post", "api", "check", "counter", "html", "debounceBlock",
  stubs + "\n" + debounceBlock + "\n" + testScript + "\n return testRun();"
);

const current0 = { id: 101, page: 1, files: [] };

await run(current0, documentMock, postStub, apiStub, check, counter, html, debounceBlock);

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("mocked fetch harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
