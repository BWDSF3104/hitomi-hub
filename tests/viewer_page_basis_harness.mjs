import { bodyIn, createChecker, readProjectFile } from "./harness_lib.mjs";

const html = readProjectFile("app", "viewer.html");
const { failures, passed, check } = createChecker();

function body(open, end) {
  return bodyIn(html, open, end);
}

const setBody = body("function setGalleryMeta(data) {", "\nfunction fileFormatFromName(name) {");
const stripItemsBody = body("function stripItems() {", "\nfunction stripGroupTotal(items) {");
const stripGroupTotalBody = body("function stripGroupTotal(items) {", "\nfunction stripGroupPage(items) {");
const stripGroupPageBody = body("function stripGroupPage(items) {", "\nfunction renderStripControls(items, group) {");
const entryForPageBody = body("function entryForPage(page) {", "\nfunction doubleActive() {");
const doubleActiveBody = body("function doubleActive() {", "\nfunction isAlreadyDoublePage(entry) {");
const isAlreadyDoublePageBody = body("function isAlreadyDoublePage(entry) {", "\nfunction displayModeFor(firstEntry) {");
const readerItemsBody = body("function readerItems() {", "\nfunction movePage(delta) {");
const movePageBody = body("function movePage(delta) {", "\nfunction fullscreenTarget() {");
const renderPositionBody = body("function renderPosition() {", "\nfunction renderLoadingPosition(id, pageLabel, pairLabel) {");
const pagenumBody = body(`document.getElementById("pagenum").addEventListener("change",`, `\ndocument.getElementById("fav").addEventListener`);
const readHelperBody = body("function runBackgroundRead(showGen, openGen) {", "async function showPage() {");

check(setBody.includes("const count = g.files.length;"), "setGalleryMeta count anchor is files basis (const count = g.files.length)");
check(!setBody.includes("sceneIndexes.length - 1"), "setGalleryMeta scene-ordinal count basis removed");

const runner = new Function("current", "stripGroup", "currentPairEnd", "window", "viewMode", "document",
  [stripItemsBody, stripGroupTotalBody, stripGroupPageBody, entryForPageBody, doubleActiveBody, isAlreadyDoublePageBody, readerItemsBody, movePageBody, renderPositionBody].join("\n") +
  "\nreturn { stripItems, stripGroupTotal, stripGroupPage, entryForPage, readerItems, movePage, renderPosition, doubleActive };");

// 733697-shape sparse fixture: 210 files, 13 scenes, sentinel = files length. Entries portrait (width <= height).
const files = Array.from({ length: 210 }, (_, i) => ({ hash: "f" + i, width: 600, height: 800, local: false }));
// last scene starts at file idx 201 (page 202) so the final file page 210 is mid-scene, as in real 733697 (sparse)
const sceneIndexes = [0, 1, 31, 60, 90, 120, 150, 170, 186, 191, 196, 199, 201, 210];

function makeEnv(page, viewMode = "single", fixture = { files, sceneIndexes }) {
  const positionText = { textContent: "" };
  const document = { getElementById: (id) => (id === "reader-position" ? positionText : null) };
  const countLine = setBody.match(/const count = [^\n]+;/)[0];
  const count = new Function("g", countLine + "\nreturn count;")({ files: fixture.files, scene_indexes: fixture.sceneIndexes });
  const env = {
    current: { id: "733697", page, count, files: fixture.files, sceneIndexes: fixture.sceneIndexes },
    stripGroup: null, currentPairEnd: null, window: { innerWidth: 1666 }, viewMode,
  };
  const api = runner(env.current, env.stripGroup, env.currentPairEnd, env.window, env.viewMode, document);
  return { env, api, positionText };
}

// executed count through the actual setGalleryMeta line
const env1 = makeEnv(1);
check(env1.env.current.count === 210, "count executed from actual setGalleryMeta line is 210 for 733697-shape fixture (files 210, scenes 13)");

// position display through actual renderPosition lines
env1.api.renderPosition();
check(env1.positionText.textContent === "位置: 1 / 210", "position display denominator is 210 at page 1 through actual renderPosition lines");
const envPair = makeEnv(32, "double");
envPair.api.renderPosition();
check(envPair.positionText.textContent === "位置: 32-33 / 210", "position display shows pair 32-33 over denominator 210 in double mode at scene start page 32");

// movePage reaches the final file page 210 on the unified count (single mode)
let reached = 1;
let moves = 0;
while (env1.api.movePage(1)) {
  reached = env1.env.current.page;
  moves++;
  if (moves > 250) break;
}
check(reached === 210 && env1.api.entryForPage(210) === files[209] && !env1.api.movePage(1), "movePage on the unified count reaches final file page 210 and stops there (single mode)");

// pagenum guard through the actual condition line
const guard = new Function("current", "page", "return (" + (pagenumBody.match(/page >= 1 && page <= current\.count/)[0]) + ");");
check(guard(env1.env.current, 210) && !guard(env1.env.current, 211), "pagenum guard through actual condition accepts 210 and rejects 211");

// read complete through the actual helper lines
const completeLines = [
  readHelperBody.match(/const lastPage = [^\n]+;/)[0],
  readHelperBody.match(/const lastShown = [^\n]+;/)[0],
  readHelperBody.match(/const complete = lastPage > 0 && lastShown >= lastPage;/)[0],
].join("\n");
const completeAt = new Function("current", "currentPairEnd", "doubleActive", completeLines + "\nreturn complete;");
check(completeAt(env1.env.current, null, () => false) === true, "read complete through actual helper lines is true at last file page 210");
check(completeAt({ ...env1.env.current, page: 13 }, null, () => false) === false, "read complete through actual helper lines is false at page 13 while final file is 210");

// readerItems second on the files basis
const items32 = envPair.api.readerItems();
check(items32.length === 2 && items32[1].page === 33 && items32[1].entry === files[32], "readerItems second is next file page 33 at sparse scene start page 32, not the next scene start page 61");
const envMid = makeEnv(62, "double");
const itemsMid = envMid.api.readerItems();
check(itemsMid.length === 2 && itemsMid[1].page === 63 && itemsMid[1].entry === files[62], "readerItems second is next file page 63 at mid-scene page 62");

// strip items are files-basis (recommended A: items = all files) and the scene badge display is kept
const strip = env1.api.stripItems();
check(strip.length === 210 && strip[1].page === 2 && strip[1].badge === 30 && strip[32].page === 33 && strip[32].badge === 0, "strip items files-basis (210) with scene badge kept: badge 30 at scene start page 2, badge 0 at mid-scene page 33");

// strip group resolves for the final file page 210 on the files basis (prefetch queue covers all 210)
const group210 = env1.api.stripGroupPage(strip);
const total = env1.api.stripGroupTotal(strip);
const start = (group210 - 1) * 50;
const end = Math.min(start + 50, strip.length);
check(group210 === 5 && group210 >= 1 && group210 <= total && start < end && start <= 209 && end > 209, "stripGroupPage at final file page 210 resolves group 5 within total 5 and the final file card is rendered (strip non-empty)");

// 1:1 + sentinel fixture behavior unchanged
const small1to1 = { files: [files[0], files[1], files[2]], sceneIndexes: [0, 1, 2, 3] };
const env11 = makeEnv(1, "double", small1to1);
check(env11.env.current.count === 3 && env11.api.readerItems()[1].page === 2 && env11.api.readerItems()[1].entry === files[1], "1:1 + sentinel fixture unchanged: count 3, second at page 1 is next file page 2");

// no-scene gallery behavior unchanged
const envPlain = makeEnv(1, "double", { files: [files[0], files[1], files[2]], sceneIndexes: [] });
check(envPlain.env.current.count === 3 && envPlain.api.readerItems()[1].page === 2 && envPlain.api.readerItems()[1].entry === files[1], "no-scene gallery unchanged: count 3, second at page 1 is next file page 2");

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("viewer page basis harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
