import { createChecker, readProjectFile, bodyIn } from "./harness_lib.mjs";

const viewer = readProjectFile("app", "viewer.html");
const { failures, passed, check } = createChecker();

function extract(source, name) {
  const re = new RegExp("^function " + name + "\\([^)]*\\) \\{[\\s\\S]*?^\\}", "m");
  const m = source.match(re);
  if (!m) throw new Error("function not found in app source: " + name);
  return m[0];
}

const startupBlock = bodyIn(
  viewer,
  'const spreadButton = document.getElementById("spread-direction");\nif (spreadButton) spreadButton',
  "const initial = parseReaderHash"
);

const defaultDir = (viewer.match(/let spreadDirection = "(rtl|ltr)";/) || [])[1];
if (!defaultDir) throw new Error("default pin let spreadDirection is missing");

const KEY = "hitomi_hub_spread_direction";

function harness(storage, initialDir) {
  const button = { textContent: null };
  const documentMock = { getElementById: (id) => (id === "spread-direction" ? button : null) };
  const entries = new Map([...storage]);
  const localStorageMock = {
    getItem: (k) => (entries.has(k) ? entries.get(k) : null),
    setItem: (k, v) => entries.set(k, v),
  };
  const run = new Function(
    "localStorage", "document", "spreadDirection",
    ["loadSpreadDirection", "setSpreadDirection", "spreadButtonText"]
      .map((name) => extract(viewer, name))
      .join("\n") +
    "\nconst SPREAD_DIRECTION_STORAGE_KEY = \"hitomi_hub_spread_direction\";\n" +
    "\nfunction renderReader() {}\nfunction renderPosition() {}\nfunction renderViewState() {}\nfunction startFullPrefetch() {}\n" +
    "\nloadSpreadDirection();\n" +
    startupBlock +
    "\nreturn { get spreadDirection() { return spreadDirection; }, button: spreadButton, setSpreadDirection };"
  )(localStorageMock, documentMock, initialDir);
  return { run, entries };
}

const def = harness([], defaultDir);
check(def.run.button.textContent === "右→左", "startup assignment renders Japanese label for default rtl (no raw label from startup block)");
check(def.run.spreadDirection === "rtl", "internal value stays raw rtl after startup");

const ltr = harness([[KEY, "ltr"]], defaultDir);
check(ltr.run.button.textContent === "左→右", "startup assignment renders Japanese label for stored ltr");
check(ltr.run.spreadDirection === "ltr", "internal value stays raw ltr from storage round-trip");

const beforeLabel = String(defaultDir);
check(beforeLabel === "rtl" && def.run.button.textContent !== beforeLabel, "before raw assignment would show raw label; fixed startup cannot show a raw label");

const tog = harness([], defaultDir);
tog.run.setSpreadDirection(tog.run.spreadDirection === "rtl" ? "ltr" : "rtl");
check(tog.run.spreadDirection === "ltr" && tog.entries.get(KEY) === "ltr", "toggle keeps internal raw value and stores raw value (saved value meaning unchanged)");
check(tog.run.button.textContent === "左→右", "toggle renders Japanese label after switch (display behavior unchanged)");

check(viewer.includes('let spreadDirection = "rtl";'), 'default pin let spreadDirection = "rtl"; unchanged');
check(viewer.includes('const SPREAD_DIRECTION_STORAGE_KEY = "hitomi_hub_spread_direction";'), "storage key pin unchanged");
check(viewer.includes("localStorage.setItem(SPREAD_DIRECTION_STORAGE_KEY, spreadDirection);"), "setSpreadDirection stores the raw internal value (not the label)");
check(viewer.includes('if (dir === "rtl" || dir === "ltr") spreadDirection = dir;'), "loadSpreadDirection still accepts only raw values");
check(viewer.includes('return dir === "rtl" ? "右→左" : "左→右";'), "spreadButtonText mapping meaning unchanged (rtl=右→左・ltr=左→右)");
check(startupBlock.includes("spreadButtonText(spreadDirection)") && !startupBlock.includes("= spreadDirection;"), "startup block maps through spreadButtonText (raw assignment removed)");

if (failures.length) {
  for (const f of failures) console.log("FAIL " + f);
  process.exit(1);
}
console.log("n19 spread button label harness: " + passed.length + " checks passed");
for (const p of passed) console.log("PASS " + p);
