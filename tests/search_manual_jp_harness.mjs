import { bodyIn, createChecker, readProjectFile } from "./harness_lib.mjs";

const searchHtml = readProjectFile("app", "search.html");

const { failures, passed, check } = createChecker();

const manualBlock = bodyIn(searchHtml, "const manualSearchInFlightKeys = new Set();", "function badgeFlag(label, className) {");
const schedulerBlock = bodyIn(searchHtml, "const PREFETCH_IDLE_MS = 800;", "document.getElementById(\"search\").addEventListener");
const renderBody = bodyIn(searchHtml, "function renderSearchCard(r, opts = {}) {", "function searchCardCoverSource(card) {");
const wiringBlock = searchHtml.slice(searchHtml.indexOf("document.getElementById(\"search\").addEventListener"));

check(
  schedulerBlock.includes("const PREFETCH_AHEAD_PAGES = 2;") && schedulerBlock.includes("const PREFETCH_CONCURRENCY = 4;") &&
  schedulerBlock.includes("const PREFETCH_IDLE_MS = 800;") && schedulerBlock.includes("const PREFETCH_COOLDOWN_MS = 300000;") &&
  !schedulerBlock.includes("manualSearch"),
  "existing prefetch constants are kept at 2 / 4 / 800 / 300000 and the manual helper stays outside the scheduler block"
);
check(renderBody.includes("manualSearchButtonHTML(r)") && renderBody.includes(" + manualButton + \"</div>") && manualBlock.includes("return \" <button"),
  "manual button renders inline on the existing .meta row without a new row");
check(!manualBlock.includes("localStorage") && !manualBlock.includes("post(") && !manualBlock.includes("saveSearchState") &&
  !manualBlock.includes("/api/thumb/") && !manualBlock.includes("innerHTML"),
  "manual path never touches saved query storage, TN requests, or innerHTML");
check(wiringBlock.includes("document.getElementById(\"results\").addEventListener(\"click\", (e) => { handleManualJapaneseSearchClick(e); });"),
  "manual handler is wired through the results click handler");
check(manualBlock.includes("const manualSearchInFlightKeys = new Set();") && manualBlock.includes("manualSearchInFlightKeys.has(key)"),
  "duplicate clicks on the same normalized key are guarded by the in-flight map");

const stubs = `
let searchGeneration = 1;
let fakeNow = 1000;
let hook = null;
let postCalls = 0;
let savedQueryChangeCalls = 0;
let queryInputValue = "female:yuri";
let prefetchCooldownUntil = 0;
let scenarioIds = [];
let scenarioCards = {};
const requests = [];
const titleGroups = new Map();
let newTabMode = false;
function normalizeTitleKey(r) {
  return String(r.title || r.japanese_title || r.id || "").trim().toLowerCase();
}
function setSearchTitleGroup(r) {
  const key = normalizeTitleKey(r);
  if (!key || r.id === undefined) return;
  const existing = titleGroups.get(key);
  if (!existing || existing.language !== "japanese" || r.language === "japanese") {
    titleGroups.set(key, { bestId: r.id, language: r.language || "", name: r.japanese_title || r.title || String(r.id) });
  }
}
function viewerLink(id) {
  return "/#/g/" + id;
}
const Date = { now: () => fakeNow };
async function api(p) {
  requests.push(p);
  if (hook) { const r = hook(p); if (r) await r; }
  await Promise.resolve();
  const path = p.split("?")[0];
  if (path === "/api/search") return { ids: scenarioIds, total: scenarioIds.length };
  if (path.startsWith("/api/search-card/")) {
    const id = Number(path.slice("/api/search-card/".length));
    return scenarioCards[id] || { id };
  }
  return {};
}
async function post() { postCalls++; return {}; }
function saveSearchState() { savedQueryChangeCalls++; }
function fakeRow() { return { kind: "div", children: [], appendChild(n) { this.children.push(n); } }; }
function fakeButton(dataset, row) {
  const btn = { kind: "button", dataset, disabled: false, removed: false, row };
  btn.closest = (sel) => sel.indexOf("data-manual-jp-search") >= 0 ? btn : (sel === ".meta" ? row : null);
  btn.remove = () => { btn.removed = true; };
  return btn;
}
const document = {
  getElementById: (id) => ({ value: id === "query" ? queryInputValue : "" }),
  createElement: (tag) => ({ kind: tag, className: "", textContent: "", href: "", target: "" }),
  createTextNode: (t) => ({ kind: "text", text: t }),
};
function resetScenario(gen) {
  searchGeneration = gen;
  fakeNow = 1000;
  hook = null;
  postCalls = 0;
  savedQueryChangeCalls = 0;
  queryInputValue = "female:yuri";
  prefetchCooldownUntil = 0;
  scenarioIds = [];
  scenarioCards = {};
  requests.length = 0;
  titleGroups.clear();
  manualSearchInFlightKeys.clear();
}
`;

const testScript = `
async function runAll() {
  check(manualJapaneseSearchTerm({ japanese_title: "Kemo Nee-san", title: "x" }) === "kemo nee-san" &&
    manualJapaneseSearchTerm({ japanese_title: "", title: "Big_Breasts" }) === "big breasts",
    "manual term helper prefers japanese_title and mirrors lowercase and underscore to space");
  check(manualSearchQueryUrl("big breasts") === "/api/search?query=big%20breasts%20language%3Ajapanese&lang=all&limit=1" &&
    manualSearchQueryUrl("furry").indexOf("lang=japanese") < 0,
    "query builder is the canonical exact term plus language condition with lang=all and limit=1, not lang=japanese only");

  // scenario a: found flow applies the group and replaces the button on the clicked row only
  resetScenario(1);
  scenarioIds = [201];
  scenarioCards = { 201: { id: 201, language: "japanese", title: "Furry" } };
  const rowA = fakeRow();
  const btnA = fakeButton({ key: "furry", term: "furry", state: "idle" }, rowA);
  await handleManualJapaneseSearchClick({ target: btnA });
  check(requests.length === 2 && requests[0] === "/api/search?query=furry%20language%3Ajapanese&lang=all&limit=1" &&
    requests[1] === "/api/search-card/201",
    "manual path requests only /api/search and /api/search-card for the clicked card");
  check(!requests.some((p) => p.indexOf("/api/thumb/") >= 0), "manual path issues no TN request");
  check(postCalls === 0 && savedQueryChangeCalls === 0 && queryInputValue === "female:yuri",
    "manual path does not change the saved query");
  check(btnA.removed && btnA.dataset.state === "found" && rowA.children.length === 2 &&
    rowA.children[0].text === "同名の日本語版: " && rowA.children[1].kind === "a" && rowA.children[1].href === "/#/g/201",
    "found state replaces the button with the link on the clicked row only");
  check(titleGroups.get("furry") && Number(titleGroups.get("furry").bestId) === 201 && titleGroups.get("furry").language === "japanese",
    "the title group is applied only for a Japanese result card without error");

  // scenario b: stale generation does not update the anchor after the fetch
  resetScenario(1);
  scenarioIds = [202];
  hook = (p) => { if (requests.length === 1) searchGeneration = 9; };
  const rowB = fakeRow();
  const btnB = fakeButton({ key: "furry", term: "furry", state: "idle" }, rowB);
  await handleManualJapaneseSearchClick({ target: btnB });
  check(requests.length === 1 && rowB.children.length === 0 && btnB.dataset.state === "busy" && !titleGroups.has("furry"),
    "stale search generation does not update the clicked card anchor");

  // scenario c: busy state during in-flight and duplicate click stop on the same normalized key
  resetScenario(1);
  scenarioIds = [203];
  scenarioCards = { 203: { id: 203, language: "japanese", title: "Furry" } };
  const rowC1 = fakeRow();
  const btnC1 = fakeButton({ key: "furry", term: "furry", state: "idle" }, rowC1);
  const rowC2 = fakeRow();
  const btnC2 = fakeButton({ key: "furry", term: "furry", state: "idle" }, rowC2);
  let mid = null;
  let second = null;
  hook = (p) => {
    if (requests.length === 1) {
      mid = { state: btnC1.dataset.state, disabled: btnC1.disabled };
      second = handleManualJapaneseSearchClick({ target: btnC2 });
    }
  };
  await handleManualJapaneseSearchClick({ target: btnC1 });
  await second;
  check(mid && mid.state === "busy" && mid.disabled,
    "busy state disables the button while the foreground fetch is in flight");
  check(requests.length === 2 && btnC2.disabled && btnC2.dataset.state === "busy" && !btnC2.removed && rowC2.children.length === 0,
    "duplicate click on the same normalized key is stopped by the in-flight map with busy state");

  // scenario d: colon title is unsupported without fetch
  resetScenario(1);
  const rowD = fakeRow();
  const btnD = fakeButton({ key: "test:colon", term: "test:colon", state: "idle" }, rowD);
  await handleManualJapaneseSearchClick({ target: btnD });
  check(requests.length === 0 && rowD.children.length === 1 && String(rowD.children[0].textContent).indexOf("unsupported") >= 0 &&
    btnD.dataset.state === "none",
    "colon title click is an unsupported message without fetch");

  // scenario e: known Japanese group replaces the button with the link without fetch
  resetScenario(1);
  titleGroups.set("furry", { bestId: 777, language: "japanese", name: "furry" });
  const rowE = fakeRow();
  const btnE = fakeButton({ key: "furry", term: "furry", state: "idle" }, rowE);
  await handleManualJapaneseSearchClick({ target: btnE });
  check(requests.length === 0 && btnE.removed && rowE.children.length === 2 && rowE.children[1].kind === "a" &&
    rowE.children[1].href === "/#/g/777",
    "known Japanese group click replaces the button with the link without fetch");

  // scenario f: cooldown is respected before fetching, then fetch proceeds
  resetScenario(1);
  prefetchCooldownUntil = fakeNow + 5000;
  scenarioIds = [204];
  scenarioCards = { 204: { id: 204, language: "japanese", title: "Furry" } };
  const rowF = fakeRow();
  const btnF = fakeButton({ key: "furry", term: "furry", state: "idle" }, rowF);
  await handleManualJapaneseSearchClick({ target: btnF });
  check(requests.length === 0 && rowF.children.length === 1 && btnF.dataset.state === "error",
    "manual click respects the prefetch cooldown before fetching");
  fakeNow += 5001;
  await handleManualJapaneseSearchClick({ target: btnF });
  check(requests.length === 2 && rowF.children[2].kind === "a" && rowF.children[2].href === "/#/g/204",
    "manual fetch proceeds after the cooldown expires");

  // error state keeps the message on the same row (no re-render)
  resetScenario(1);
  hook = async (p) => {
    const err = new Error("502 " + p);
    err.status = 502;
    throw err;
  };
  const rowG = fakeRow();
  const btnG = fakeButton({ key: "furry", term: "furry", state: "idle" }, rowG);
  await handleManualJapaneseSearchClick({ target: btnG });
  check(requests.length === 1 && btnG.dataset.state === "error" && rowG.children.length === 1 &&
    String(rowG.children[0].textContent).indexOf("日本版検索エラー") >= 0 && !manualSearchInFlightKeys.has("furry"),
    "error state shows the message on the same row and releases the in-flight key");
}
`;

const run = new Function("check", stubs + "\n" + manualBlock + "\n" + testScript + "\n return runAll();");

await run(check, null);

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("manual Japanese search harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
