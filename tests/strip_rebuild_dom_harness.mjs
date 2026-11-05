import { bodyIn, createChecker, readProjectFile } from "./harness_lib.mjs";

const html = readProjectFile("app", "viewer.html");

function extract(name) {
  const re = new RegExp("^function " + name + "\\([^)]*\\) \\{[\\s\\S]*?^\\}", "m");
  const m = html.match(re);
  if (!m) throw new Error("function not found in app/viewer.html: " + name);
  return m[0];
}

const { failures, passed, check } = createChecker();

const counter = { rebuilds: 0, nodeSeq: 0 };

function parseImgNodes(htmlStr) {
  const nodes = [];
  for (const tag of htmlStr.matchAll(/<img[^>]*>/g)) {
    const pageMatch = tag[0].match(/data-page='(\d+)'/);
    const classMatch = tag[0].match(/class='([^']*)'/);
    if (!pageMatch) continue;
    const classes = new Set(classMatch && classMatch[1] ? classMatch[1].split(" ") : []);
    nodes.push({
      nodeId: ++counter.nodeSeq,
      dataset: { page: pageMatch[1] },
      classList: {
        toggle(cls, force) {
          if (force === undefined) { if (classes.has(cls)) classes.delete(cls); else classes.add(cls); }
          else if (force) classes.add(cls);
          else classes.delete(cls);
        },
        contains: (cls) => classes.has(cls),
      },
    });
  }
  return nodes;
}

const thumbs = {
  _html: "",
  childNodes: [],
  get innerHTML() { return this._html; },
  set innerHTML(v) {
    counter.rebuilds++;
    this._html = v;
    this.childNodes = parseImgNodes(v);
  },
};

const mockElements = {
  thumbs,
  "strip-group": { value: "" },
  "strip-group-status": { textContent: "" },
};

const documentMock = {
  getElementById: (id) => mockElements[id] || null,
  querySelectorAll: (sel) => sel === "#thumbs img" ? thumbs.childNodes : [],
};

const stubs = `
const currentGalleryObjectUrlKeys = new Set();
const sourceStatusByEntry = new Map();
const fullCache = new Map();
function readerItems() { return [{ page: current.page, entry: current.files[current.page - 1] }]; }
function fullCacheKey(entry, galleryId) { return galleryId + "|" + entry.hash; }
function entrySourceType(entry, galleryId) { return "tn"; }
function stripThumbSource(entry, galleryId) { return "/api/image/" + entry.hash + "?format=webp&gallery=" + galleryId; }
function setSourceType() {}
function setSourceStatus() {}
function markGalleryObjectUrlKey() {}
function route(id, page) { return "#/g/" + id + "/page/" + page; }
function targetAttr() { return ""; }
function escapeHTML(s) { return String(s); }
function attachDisplayLoadHandlers() {}
`;

const testScript = `
function testRun() {
  const nodeByPage = (p) => thumbs.childNodes.find((n) => Number(n.dataset.page) === p);

  current.files = Array.from({ length: 130 }, (_, i) => ({ hash: "h" + i, width: 100, height: 200, local: false }));
  current.page = 1;

  check(OPEN_BODY.includes("stripRenderedGallery = null;") && OPEN_BODY.includes("stripRenderedGroup = null;"), "openGallery resets strip rebuild state on open");
  check(!OPEN_BODY.includes("renderStrip();"), "openGallery does not rebuild the strip again after showPage");
  check(SHOW_BODY.includes("refreshStrip();") && !SHOW_BODY.includes("renderStrip();"), "showPage updates strip via refreshStrip only");

  const rebuiltOpen = refreshStrip();
  check(rebuiltOpen === true && counter.rebuilds === 1, "gallery open does full rebuild once");
  check(thumbs.childNodes.length === 50, "open renders group window of 50 img nodes");
  check(nodeByPage(1) && nodeByPage(1).classList.contains("current"), "active class on current page after open");

  const snapshot = [...thumbs.childNodes];
  current.page = 23;
  const rebuiltMove = refreshStrip();
  check(rebuiltMove === false && counter.rebuilds === 1, "page movement does not regenerate strip img nodes");
  check(thumbs.childNodes.length === snapshot.length, "strip child count unchanged on page movement");
  check(thumbs.childNodes.every((n, i) => n === snapshot[i]), "strip img nodes are the same objects (not regenerated)");
  check(nodeByPage(23).classList.contains("current") && !nodeByPage(1).classList.contains("current"), "active class reflects page movement on existing nodes");

  current.page = 31;
  refreshStrip();
  check(counter.rebuilds === 1 && nodeByPage(31).classList.contains("current") && nodeByPage(23).classList.contains("current") === false && thumbs.childNodes.length === snapshot.length && thumbs.childNodes[0] === snapshot[0], "second page movement still highlight-only");

  current.page = 60;
  refreshStrip();
  check(counter.rebuilds === 2, "group window change rebuilds only when the rendered window changes");

  current.id = 202;
  current.page = 2;
  stripGroup = null;
  stripRenderedGallery = null;
  stripRenderedGroup = null;
  refreshStrip();
  check(counter.rebuilds === 3, "next gallery open does full rebuild exactly once");
  current.page = 5;
  refreshStrip();
  check(counter.rebuilds === 3, "page movement after open stays highlight-only");
}
`;

const run = new Function(
  "current", "stripGroup", "stripRenderedGallery", "stripRenderedGroup",
  "document", "check", "OPEN_BODY", "SHOW_BODY", "thumbs", "counter",
  stubs +
  ["stripItems", "stripGroupTotal", "stripGroupPage", "renderStripControls",
   "highlightStrip", "renderStrip", "refreshStrip"].map(extract).join("\n") +
  "\n" + testScript + "\n testRun();"
);

const showPageBody = bodyIn(html, "async function showPage() {", "\nfunction renderStrip() {");
const openGalleryBody = bodyIn(html, "async function openGallery(id, page, second) {", "\nfunction renderStrip() {");
const renderStripBody = bodyIn(html, "function renderStrip() {", "\nfunction highlightStrip() {");
const highlightBody = bodyIn(html, "function highlightStrip() {", "\nfunction refreshStrip() {");

check(renderStripBody.includes("stripRenderedGallery = current.id;") && renderStripBody.includes("stripRenderedGroup = group;"), "renderStrip records the rendered gallery and group window");
check(highlightBody.includes('classList.toggle("current"'), "highlightStrip updates active class on existing img nodes");

const current0 = { id: 101, page: 1, count: 130, files: [] };
run(current0, null, null, null, documentMock, check, openGalleryBody, showPageBody, thumbs, counter);

check(html.includes("function stripThumbSource(entry, galleryId = current.id) {\n  return displayViewerSource(entry, galleryId);\n}"), "TN display source rule unchanged");
check(html.includes('" | TN表示フォールバック禁止"'), "TN display fallback policy text unchanged");

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("mocked DOM harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
