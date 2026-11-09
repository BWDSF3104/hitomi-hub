import { createChecker, readProjectFile } from "./harness_lib.mjs";

const viewer = readProjectFile("app", "viewer.html");
const search = readProjectFile("app", "search.html");
const { failures, passed, check } = createChecker();

function extract(source, name) {
  const re = new RegExp("^function " + name + "\\([^)]*\\) \\{[\\s\\S]*?^\\}", "m");
  const m = source.match(re);
  if (!m) throw new Error("function not found in app source: " + name);
  return m[0];
}

function decodeEntity(value) {
  return value
    .replace(/&#39;/g, "'")
    .replace(/&quot;/g, '"')
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&amp;/g, "&");
}

function attributesOf(tag) {
  const out = new Map();
  for (const m of tag.matchAll(/([A-Za-z0-9_-]+)='([^']*)'/g)) out.set(m[1], m[2]);
  return out;
}

const HEX = "899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9";
const PAYLOAD = "abc' onerror='window.__n15 = 1' data-x='y";
const ENTRIES = [
  { hash: HEX, local: false, local_formats: [], hasavif: false },
  { hash: PAYLOAD, local: false, local_formats: [], hasavif: false },
  { hash: "h3", local: false, local_formats: [], hasavif: false },
];

const viewerStubs = `
const galleryFormats = new Map();
function entryFormat(entry, galleryId) { return entry.hasavif ? "avif" : (galleryFormats.get(galleryId) || current.format || "webp"); }
function fullCacheKey(entry, galleryId) { return galleryId + "|" + entry.hash + "|" + entryFormat(entry, galleryId); }
function readerClass() { return ""; }
function readerItems() { return current.files.map((entry, idx) => ({ page: idx + 1, entry })); }
function stripItems() { return current.files.map((entry, idx) => ({ page: idx + 1, entry, badge: 0 })); }
function stripGroupTotal(items) { return 1; }
function stripGroupPage(items) { return 1; }
function renderStripControls() {}
function highlightStrip() {}
function setSourceType() {}
function setSourceStatus() {}
function markDisplayObjectUrlKey() {}
function markGalleryObjectUrlKey() {}
function attachDisplayLoadHandlers() {}
function entrySourceType() { return "tn"; }
function displayViewerSource(entry, galleryId) { return "/api/image/" + encodePathSegment(entry.hash) + "?format=" + entryFormat(entry, galleryId); }
function stripThumbSource(entry, galleryId) { return displayViewerSource(entry, galleryId); }
function route(id, page) { return "#/g/" + id + "/page/" + page; }
function targetAttr() { return ""; }
`;

const viewerModule = new Function(
  "escapeHTML", "encodePathSegment", "current", "document",
  "stripGroup", "stripRenderedGallery", "stripRenderedGroup", "renderGeneration",
  "spreadDirection", "currentDisplayObjectUrlKeys", "currentGalleryObjectUrlKeys",
  viewerStubs +
  ["entrySource", "renderReader", "renderStrip", "updateThumbCache"].map((name) => extract(viewer, name)).join("\n") +
  "\nreturn { entrySource, renderReader, renderStrip, updateThumbCache, escapeHTML, encodePathSegment };"
);

function nodesFrom(html) {
  return [...html.matchAll(/<img[^>]*>/g)].map((tag) => {
    const attrs = attributesOf(tag[0]);
    return { src: decodeEntity(attrs.get("src") || ""), dataset: Object.fromEntries(attrs) };
  });
}

function runViewer(escapeImpl, encodeImpl) {
  const captured = {};
  const current = { id: 101, page: 1, format: "webp", files: ENTRIES };
  const documentMock = {
    getElementById: (id) => ({ set innerHTML(v) { captured[id] = v; }, className: "" }),
    querySelectorAll: (sel) => sel === "#thumbs img" ? nodesFrom(captured.thumbs) : [],
  };
  const mod = viewerModule(escapeImpl, encodeImpl, current, documentMock, 1, 1, 1, 0, "ltr", new Set(), new Set());
  mod.renderStrip();
  mod.renderReader();
  const strip = [...captured.thumbs.matchAll(/<img[^>]*>/g)].map((m) => attributesOf(m[0]));
  const reader = [...captured.reader.matchAll(/<img[^>]*>/g)].map((m) => attributesOf(m[0]));
  return { mod, current, strip, reader };
}

const identityEscape = (s) => String(s);
const identityEncode = (s) => String(s);
const realEscape = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const realEncode = (s) => encodeURIComponent(String(s ?? "")).replace(/'/g, "%27");

const before = runViewer(identityEscape, identityEncode);
const after = runViewer(realEscape, realEncode);

check(before.strip.length === 3 && before.reader.length === 3, "before renders 3 strip img and 3 reader img nodes");
check(after.strip.length === 3 && after.reader.length === 3, "after renders the same node count (3 strip, 3 reader)");

check(before.strip.some((a) => a.has("onerror") && a.has("data-x")) && before.reader.some((a) => a.has("onerror") && a.has("data-x")), "before breaks the single-quote attribute boundary (injected onerror attribute appears)");
check(!after.strip.some((a) => a.has("onerror") || a.has("data-x")) && !after.reader.some((a) => a.has("onerror") || a.has("data-x")), "after keeps the attribute boundary (no injected attribute)");

check(before.strip[1].get("data-hash") === "abc" && decodeEntity(after.strip[1].get("data-hash")) === PAYLOAD, "before truncates data-hash to abc, after round-trips the full value");
check(after.strip.every((a, i) => decodeEntity(a.get("data-hash")) === ENTRIES[i].hash), "after data-hash round-trips for every entry");
check(after.strip.every((a, i) => decodeEntity(a.get("data-key")) === "101|" + ENTRIES[i].hash + "|webp"), "after data-key round-trips for every entry");
check(after.strip.every((a) => a.size === after.strip[0].size), "after attribute count per img is unchanged across entries (boundary preserved)");

check(after.mod.entrySource(ENTRIES[0], 101) === "/api/image/" + HEX + "?format=webp&gallery=101", "entrySource bytes for a valid 64 hex hash are unchanged");
check(!after.mod.entrySource(ENTRIES[1], 101).includes("'") && !before.mod.entrySource(ENTRIES[1], 101).includes("%27"), "entrySource encodes ' to %27 (before left it raw in the path)");
check(decodeURIComponent(after.mod.encodePathSegment(PAYLOAD)) === PAYLOAD, "path segment percent encoding round-trips");

const payloadKey = "101|" + PAYLOAD + "|webp";
check(after.strip.some((a) => decodeEntity(a.get("data-key")) === payloadKey), "after dataset compare resolves the payload entry key");
check(!before.strip.some((a) => decodeEntity(a.get("data-key")) === payloadKey), "before dataset compare cannot resolve the payload entry key (value truncated)");

check(viewer.includes("el.dataset.key === key") && viewer.includes("el.dataset.hash === entry.hash"), "selector strings no longer embed remote values (dataset compare)");

check(viewer.includes(`data-hash='`) && viewer.includes(`data-key='`), "literal pins data-hash=' and data-key=' are still present");
check(viewer.includes("function entrySource(entry, galleryId = current.id, force = false) {") && viewer.includes("const fullSrc = entrySource(entry, galleryId, force);"), "entrySource pins are unchanged");

const searchEscape = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const searchHelper = new Function(
  "newTabMode",
  extract(search, "escapeHTML") + "\n" + extract(search, "encodeURIComponentSingleQuote") + "\n" +
  extract(search, "targetAttr") + "\n" + extract(search, "parseSearchHash") + "\n" +
  "return { escapeHTML, encodeURIComponentSingleQuote, targetAttr, parseSearchHash };"
)(false);

const queries = [
  { name: "saved", query: "galleries" },
  { name: "corner", query: "don't steal" },
  { name: "plain", query: "tag name" },
  { name: "quote", query: "single'quote" },
];

const listHtml = queries.map((q) =>
  "<li><a href='#/q/" + searchHelper.encodeURIComponentSingleQuote(q.query) + "'" + searchHelper.targetAttr() + ">" + searchHelper.escapeHTML(q.name) + "</a> <span class='meta'>" + searchHelper.escapeHTML(q.query) + "</span></li>").join("");

check(search.includes("encodeURIComponentSingleQuote(q.query)"), "search queries href uses the single-quote helper");
check((listHtml.match(/href='[^']*'/g) || []).length === 4, "search renders exactly one href attribute per saved query");
check(queries.every((q) => searchHelper.parseSearchHash("#/q/" + searchHelper.encodeURIComponentSingleQuote(q.query)).query === q.query), "search route round-trips for every query including the ones with '");
check(searchHelper.encodeURIComponentSingleQuote("galleries") === encodeURIComponent("galleries"), "link meaning is unchanged for queries without '");

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("n15 attribute escape harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
