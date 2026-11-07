import { bodyIn, createChecker, readProjectFile } from "./harness_lib.mjs";

const html = readProjectFile("app", "viewer.html");

const { failures, passed, check } = createChecker();

// string-level pins (R-15)
check(html.includes("let MAX_FULL_CACHE_ENTRIES = 4000;"), "objectURL cap default is the approved 4000");
check(!html.includes("MAX_FULL_CACHE_ENTRIES = 2000"), "the old fixed 2000 cap is removed");
check(html.includes('fetch("/api/viewer-config", { cache: "no-store" })'), "the cap is passed to the page through the server config route");
check(html.includes("const cap = Number(cfg && cfg.max_object_urls);") && html.includes("cap >= 1000"), "the page applies the cap only when the server floor guard is satisfied");
check(html.includes("function applyViewerConfigCap() {") && html.includes("applyViewerConfigCap();"), "the cap is applied at startup");
const configBlock = bodyIn(html, "function applyViewerConfigCap() {", "\napplyViewerConfigCap();");
check(!configBlock.includes("setStatus(") && !configBlock.includes("document.getElementById") && !configBlock.includes("innerHTML"), "no notification UI for the cap (users are not informed)");
check(html.includes('function savedStatusEvictKey(skipKey = "") {') && html.includes('sourceStatusByEntry.get(key) === "local"'), "saved status is the first eviction basis, LRU is second");
check(html.includes("const priorityKey = savedStatusEvictKey(skipKey);"), "evictFullObjectUrls runs the status-priority pass before the LRU pass");
const evictBlock = bodyIn(html, 'function evictFullObjectUrls(skipKey = "") {', "\nfunction getGalleryCounts");
check(evictBlock.includes("if (isFullObjectUrlProtected(key)) continue;") && evictBlock.includes("const priorityKey = savedStatusEvictKey(skipKey);"), "current display and gallery keys stay protected in both passes");
const prefetchBlock = bodyIn(html, "function resetBackgroundPrefetch(displayed) {", "\nfunction ensureBackgroundPrefetch(displayed) {");
check(prefetchBlock.includes("stripItems()") && !prefetchBlock.includes("PREFETCH_WINDOW"), "prefetch keeps the whole gallery queue by distance order (window shrink not adopted)");

const blocks = [
  bodyIn(html, "function entryFormat(entry", "\nconst galleryFormats"),
  bodyIn(html, "function fullCacheKey(entry", "\nfunction cachedFullSrc"),
  bodyIn(html, "function statusIsViewerDisplaySource(status) {", "\nfunction entrySourceType"),
  bodyIn(html, "function setSourceStatus(entry", "\nfunction setSourceTypeFromKey"),
  bodyIn(html, "function setSourceTypeFromKey(key", "\nfunction setSourceType("),
  bodyIn(html, "function setSourceType(entry", "\nfunction recordImageDisplayPassFromImg"),
  bodyIn(html, "function markDisplayObjectUrlKey(key)", "\nfunction markGalleryObjectUrlKey"),
  bodyIn(html, "function markGalleryObjectUrlKey(key)", "\nfunction isFullObjectUrlProtected"),
  bodyIn(html, "function isFullObjectUrlProtected(key)", "\nfunction decrementPrefetchedCountForKey"),
  bodyIn(html, "function decrementPrefetchedCountForKey(key)", "\nfunction revokeFullObjectUrl"),
  bodyIn(html, "function revokeFullObjectUrl(key)", "\nfunction savedStatusEvictKey"),
  bodyIn(html, 'function savedStatusEvictKey(skipKey = "") {', "\nfunction evictFullObjectUrls"),
  evictBlock,
  bodyIn(html, "function getGalleryCounts(galleryId) {", "\nfunction renderRetryButtonState"),
  bodyIn(html, "function storeFullCache(entry", "\nasync function prefetchPageImage"),
];

const stubs = `
let MAX_FULL_CACHE_ENTRIES = 4000;
let current = { id: "g1" };
let backgroundGalleryId = "g1";
const galleryFormats = new Map();
const fullObjectUrls = new Set();
const fullObjectUrlLru = new Map();
const currentDisplayObjectUrlKeys = new Set();
const currentGalleryObjectUrlKeys = new Set();
const fullCache = new Map();
const prefetchedEntries = new Map();
const countsByGallery = new Map();
const displayPassByEntry = new Map();
const sourceStatusByEntry = new Map();
const sourceTypeByEntry = new Map();
const activeObjectUrlKeys = new Set();
const revoked = [];
let urlSeq = 0;
const URL = {
  createObjectURL: () => "blob:" + (urlSeq++),
  revokeObjectURL: (objectUrl) => { revoked.push(objectUrl); },
};
function updateThumbCache(key, objectUrl) {}
function renderPrefetchStatus() {}
`;

const testScript = `
async function runAll() {
  function reset(cap) {
    MAX_FULL_CACHE_ENTRIES = cap;
    current = { id: "g1" };
    backgroundGalleryId = "g1";
    fullObjectUrls.clear();
    fullObjectUrlLru.clear();
    currentDisplayObjectUrlKeys.clear();
    currentGalleryObjectUrlKeys.clear();
    fullCache.clear();
    prefetchedEntries.clear();
    countsByGallery.clear();
    displayPassByEntry.clear();
    sourceStatusByEntry.clear();
    sourceTypeByEntry.clear();
    activeObjectUrlKeys.clear();
    revoked.length = 0;
    urlSeq = 0;
  }
  function entry(hash) { return { hash, hasavif: false }; }
  function storeAndCount(e, status) {
    const objectUrl = URL.createObjectURL();
    storeFullCache(e, status, objectUrl);
    prefetchedEntries.set(fullCacheKey(e), status);
  }

  // runtime default is the approved 4000
  reset(4000);
  check(MAX_FULL_CACHE_ENTRIES === 4000, "runtime cap starts at the approved default 4000");

  // status priority first, LRU second: a saved-status (local) entry that is not the oldest
  // is evicted before the oldest cache-status entry
  reset(3);
  const e1 = entry("h1"), e2 = entry("h2"), e3 = entry("h3"), e4 = entry("h4");
  storeAndCount(e1, "cache");
  storeAndCount(e2, "local"); // saved-status entry (not the oldest)
  storeAndCount(e3, "cache");
  storeAndCount(e4, "cache"); // crossing the cap starts eviction
  const k1 = fullCacheKey(e1), k2 = fullCacheKey(e2);
  check(fullObjectUrlLru.size === 3, "the cap is enforced after eviction");
  check(!fullObjectUrlLru.has(k2) && !prefetchedEntries.has(k2) && !activeObjectUrlKeys.has(k2), "status-priority eviction removed the saved-status entry first, not the oldest");
  check(fullObjectUrlLru.has(k1), "the oldest cache-status entry is kept until LRU order reaches it");
  check(revoked.length >= 1, "the evicted objectURL is revoked");

  // no saved-status entries → plain LRU order (oldest unprotected)
  reset(2);
  const f1 = entry("g1"), f2 = entry("g2"), f3 = entry("g3");
  storeAndCount(f1, "cache");
  storeAndCount(f2, "cache");
  storeAndCount(f3, "cache");
  const q1 = fullCacheKey(f1);
  check(fullObjectUrlLru.size === 2 && !fullObjectUrlLru.has(q1), "without saved-status entries eviction is plain LRU order");

  // protected key skip: a displayed saved-status key is not evicted; the LRU pass evicts the
  // next unprotected key instead
  reset(3);
  const p0 = entry("p0"), p1 = entry("p1"), p2 = entry("p2"), p3 = entry("p3");
  storeAndCount(p0, "local");
  markDisplayObjectUrlKey(fullCacheKey(p0));
  storeAndCount(p1, "cache");
  storeAndCount(p2, "cache");
  storeAndCount(p3, "cache");
  check(fullObjectUrlLru.has(fullCacheKey(p0)), "displayed saved-status key stays protected");
  check(!fullObjectUrlLru.has(fullCacheKey(p1)), "the LRU pass evicts the oldest unprotected key while the displayed key is protected");
  check(fullObjectUrlLru.size === 3, "size never exceeds the cap after eviction");

  // cap from config is respected (the page sets MAX_FULL_CACHE_ENTRIES from /api/viewer-config)
  reset(1000);
  for (let i = 0; i < 1002; i++) storeAndCount(entry("cfg" + i), "cache");
  check(fullObjectUrlLru.size === 1000, "the configured cap (floor 1000) is respected");
}
`;

const run = new Function("check", stubs + "\n" + blocks.join("\n") + "\n" + testScript + "\n return runAll();");

await run(check);

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("viewer objectURL cap harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
