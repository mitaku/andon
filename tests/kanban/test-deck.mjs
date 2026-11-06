// Behaviour tests for kanban (the ledger's CLI), copied unchanged from herdr-flightdeck's
// herdr-app/cli/test-deck.mjs at tag deck-v0.4.0 (docs/decisions.md §7, §12); only these lines differ,
// and (Y-141) the ledger's folder kanban/, KANBAN_ROOT, chief_of_staff and four-digit ids (W-0001).
// tests/kanban.sh runs it: DECK_BIN=<path to a kanban executable> node tests/kanban/test-deck.mjs
// Without DECK_BIN it looks for bin/deck (bin/deck.exe on Windows) at the repo root. Uses a temporary folder only.
import { spawn, spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, realpathSync, rmSync, statSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const bin = resolve(process.env.DECK_BIN
  || join(dirname(fileURLToPath(import.meta.url)), "..", "..", "bin", process.platform === "win32" ? "deck.exe" : "deck"));
if (!existsSync(bin)) {
  console.error(`no deck at ${bin}: run \`go build -o bin/deck ./cmd/deck\` at the repo root, or set DECK_BIN`);
  process.exit(1);
}
const cmd = bin, pre = [];
// A yard in a temporary folder: projects/a and projects/b each hold a ledger (format 2);
// the repositories go under .ghq/ the way ghq clones them.
const yard = mkdtempSync(join(tmpdir(), "deck-test-"));
const root = join(yard, "projects", "a"), rootB = join(yard, "projects", "b");
const ghq = (name) => join(yard, ".ghq", "github.com", "x", name);
// The same folder however the OS spells it (macOS /var -> /private/var, Windows 8.3 short names).
const same = (a, b) => { try { return realpathSync.native(a) === realpathSync.native(b); } catch { return false; } };
const env = { ...process.env, KANBAN_ROOT: "", HC_TERMINAL_ID: "t9" };
delete env.KANBAN_ROOT;

const run = (args, opts = {}) => {
  const r = spawnSync(cmd, [...pre, ...args], { cwd: opts.cwd ?? root, env: { ...env, ...opts.env }, encoding: "utf8", input: opts.input });
  return { code: r.status, out: r.stdout.trim(), err: r.stderr.trim() };
};
const runAsync = (args, opts = {}) => new Promise((ok) => {
  const p = spawn(cmd, [...pre, ...args], { cwd: opts.cwd ?? root, env: { ...env, ...opts.env } });
  let out = "";
  p.stdout.on("data", (d) => (out += d));
  p.on("close", (code) => ok({ code, out: out.trim() }));
});

let failed = 0, passed = 0;
const check = (name, cond, detail = "") => {
  if (cond) passed++;
  else { failed++; console.log(`FAIL ${name}${detail ? `: ${detail}` : ""}`); }
};

try {
  mkdirSync(root, { recursive: true });
  mkdirSync(rootB, { recursive: true });
  check("init", run(["init", "--root", root, "--repo", "github.com/x/app"]).code === 0);
  check("init twice is fine", run(["init", "--root", root]).out.startsWith("already"));
  check("init in the current folder", run(["init", "--repo", "github.com/x/other"], { cwd: rootB }).code === 0);
  check("init's .gitignore is *", readFileSync(join(root, "kanban", ".gitignore"), "utf8") === "*\n");
  const cfgA = readFileSync(join(root, "kanban", "config.yaml"), "utf8");
  check("init writes version 2 and repos", cfgA.startsWith("version: 2\n") && cfgA.includes("repos:\n  - github.com/x/app\n"), cfgA);
  mkdirSync(join(root, "notes"), { recursive: true });

  const a = run(["add", "Parent work"]);
  check("add returns an id", a.out === "W-0001", a.out + a.err);
  const b = run(["add", "日本語の題名", "--parent", "W-0001", "--profile", "opus"], { cwd: join(root, "notes") });
  check("add from a folder in the project", b.out === "W-0002", b.out + b.err);
  const w2 = JSON.parse(run(["show", "W-0002", "--json"]).out).task;
  check("add --profile; no repo outside git", w2.profile === "opus" && !("repo" in w2), JSON.stringify(w2));
  check("add --repo not in repos is refused", run(["add", "x", "--repo", "github.com/x/nope"]).code === 1);
  check("unicode title kept", readFileSync(join(root, "kanban", "tasks", "W-0002", "task.yaml"), "utf8").includes("日本語の題名"));
  const c = run(["add", "After two", "--after", "W-0002"]);
  check("add with after", c.out === "W-0003");
  check("waiting task is queued", JSON.parse(run(["show", "W-0003", "--json"]).out).task.status === "queued");
  check("unknown parent is refused", run(["add", "x", "--parent", "W-0099"]).code === 1);

  const ready = run(["ready", "--json"]).out;
  check("ready has W-0001 and W-0002, not W-0003", ready.includes("W-0001") && ready.includes("W-0002") && !ready.includes("W-0003"), ready);
  check("start refuses a waiting task", run(["start", "W-0003"]).code === 1);

  const st = run(["start", "W-0002", "--json"]);
  const started = JSON.parse(st.out);
  check("start sets doing and agent", started.status === "doing" && started.agent === "t9", st.out + st.err);
  check("review", run(["review", "W-0002"]).code === 0 && JSON.parse(run(["show", "W-0002", "--json"]).out).task.status === "review");
  const d = run(["done", "W-0002"]);
  check("done frees the waiting task", d.out.includes("now ready: W-0003"), d.out + d.err);
  check("freed task moved to todo", JSON.parse(run(["show", "W-0003", "--json"]).out).task.status === "todo");

  check("set a list", run(["set", "W-0001", "links=https://a.example,docs/x.md"]).code === 0
    && JSON.parse(run(["show", "W-0001", "--json"]).out).task.links.length === 2);
  check("set removes with empty value", run(["set", "W-0001", "links="]).code === 0
    && !("links" in JSON.parse(run(["show", "W-0001", "--json"]).out).task));
  check("set refuses a bad status", run(["set", "W-0001", "status=nope"]).code === 1);
  check("set refuses id", run(["set", "W-0001", "id=W-0007"]).code === 1);
  for (const id of ["../W-0001", "..", "W-0001/../../x", "..\\W-0001"])
    check(`an id that leaves tasks/ is refused: ${id}`, run(["set", id, "title=x"]).code === 1 && run(["show", id]).code === 1);

  check("report write", run(["report", "W-0001"], { input: "# Report\nfirst\n" }).code === 0);
  check("report append", run(["report", "W-0001", "--append"], { input: "second\n" }).code === 0);
  const rep = readFileSync(join(root, "kanban", "tasks", "W-0001", "report.md"), "utf8");
  check("report has both", rep.includes("first") && rep.includes("second"), rep);

  check("list hides done by default", !run(["list"]).out.includes("W-0002") && run(["list", "--all"]).out.includes("W-0002"));
  check("root found from a subfolder", same(run(["root"], { cwd: join(root, "notes") }).out, root));
  check("lint ok", run(["lint"]).code === 0);

  // Ten writers at once: ids must not repeat and every add must be logged.
  const many = await Promise.all(Array.from({ length: 10 }, (_, i) => runAsync(["add", `parallel ${i}`])));
  const got = many.map((r) => r.out);
  check("parallel adds all succeed", many.every((r) => r.code === 0), JSON.stringify(many));
  check("parallel ids are unique", new Set(got).size === 10, got.join(" "));
  check("tasks folder has 13", readdirSync(join(root, "kanban", "tasks")).length === 13);
  const log = readFileSync(join(root, "kanban", "log.jsonl"), "utf8").trim().split("\n").map((l) => JSON.parse(l));
  check("log has every add", log.filter((l) => l.change.add).length === 13, String(log.length));
  // The lock is an OS lock on an empty file; the file stays, the lock goes with the process.
  const lockFile = join(root, "kanban", ".lock");
  check("lock is a file, not a leftover folder", !existsSync(lockFile) || statSync(lockFile).isFile());
  check("a write after the parallel adds gets the lock", run(["set", "W-0004", "phase=P1"]).code === 0);

  // A person's comments and key order in task.yaml survive set, start and done.
  const commented = join(root, "kanban", "tasks", "W-0005", "task.yaml");
  writeFileSync(commented, [
    "# Written by hand: keep this",
    "title: Commented work  # the title",
    "id: W-0005",
    "# why it waits",
    "status: todo",
    "notes: free text      # a key deck doesn't know",
    "created: 2026-11-02T10:00:00Z",
    "",
  ].join("\n"));
  const steps = [run(["set", "W-0005", "phase=P2", "links=https://a.example"]), run(["start", "W-0005"]), run(["done", "W-0005"])];
  check("commented task: set, start, done", steps.every((r) => r.code === 0), JSON.stringify(steps));
  const kept = readFileSync(commented, "utf8");
  for (const c of ["# Written by hand: keep this", "# the title", "# why it waits", "# a key deck doesn't know"])
    check(`comment kept: ${c}`, kept.includes(c), kept);
  check("key order kept", ["title:", "id:", "status:", "notes:", "created:"].every((k, i, a) => i === 0 || kept.indexOf(a[i - 1]) < kept.indexOf(k)), kept);
  check("new values written", kept.includes("phase: P2") && kept.includes("status: done") && kept.includes("https://a.example"), kept);

  // deck config set chief_of_staff= keeps a person's comments in config.yaml.
  const cfgFile = join(root, "kanban", "config.yaml");
  writeFileSync(cfgFile, readFileSync(cfgFile, "utf8").replace("version: 2\n", "# by hand\nversion: 2\n"));
  const setCo = run(["config", "set", "chief_of_staff=t7"]);
  const cfgNow = readFileSync(cfgFile, "utf8");
  check("config set chief_of_staff", setCo.code === 0 && cfgNow.startsWith("# by hand\nversion: 2\n") && cfgNow.includes("chief_of_staff: t7\n"), setCo.err + cfgNow);
  check("config set refuses other keys", run(["config", "set", "prefix=X"]).code === 1 && !readFileSync(cfgFile, "utf8").includes("prefix: X"));

  // A broken task shows up in lint and doesn't stop the rest.
  writeFileSync(join(root, "kanban", "tasks", "W-0001", "task.yaml"), "id: W-0001\ntitle: [unclosed\n");
  check("list still works with a broken task", run(["list"]).code === 0 && run(["list"]).out.includes("unreadable"));
  const lint = run(["lint"]);
  check("lint reports the broken task", lint.code === 1 && lint.out.includes("W-0001"), lint.out);

  check("no ledger is an error", run(["list"], { cwd: tmpdir() }).code === 1);

  // A version 1 ledger (no version, projects in config.yaml) is refused, and nothing is written.
  const v1 = join(yard, "v1");
  mkdirSync(join(v1, "kanban", "tasks", "W-0001"), { recursive: true });
  writeFileSync(join(v1, "kanban", "config.yaml"), "prefix: W\nprojects:\n  app: projects/app\n");
  writeFileSync(join(v1, "kanban", "tasks", "W-0001", "task.yaml"), "id: W-0001\ntitle: old\nstatus: todo\n");
  const v1Steps = [run(["add", "x"], { cwd: v1 }), run(["set", "W-0001", "status=doing"], { cwd: v1 }), run(["list"], { cwd: v1 }),
    run(["report", "W-0001"], { cwd: v1, input: "text\n" }), run(["show", "W-0001"], { cwd: v1 }), run(["root"], { cwd: v1 }),
    run(["init", "--root", v1]), run(["config", "set", "chief_of_staff=t9"], { cwd: v1 }), run(["lint"], { cwd: v1 })];
  // Y-141: a kanban/config.yaml without a version is not a ledger, so most commands say there is none.
  check("a version 1 ledger is refused", v1Steps.every((r) => r.code === 1 && /format|no ledger/.test(r.err + r.out)), JSON.stringify(v1Steps));
  check("nothing written to a version 1 ledger",
    readdirSync(join(v1, "kanban")).sort().join(",") === "config.yaml,tasks"
    && readdirSync(join(v1, "kanban", "tasks")).join(",") === "W-0001"
    && readFileSync(join(v1, "kanban", "tasks", "W-0001", "task.yaml"), "utf8").includes("status: todo"));

  // Questions: ask → questions → answer → received, no step skipped, n never reused, done stops on them.
  const qr = join(yard, "projects", "q");
  mkdirSync(qr, { recursive: true });
  const q = (args, opts = {}) => run(args, { cwd: qr, ...opts, env: { DECK_NOW: "2026-11-03T01:00:00Z", ...opts.env } });
  const qfile = (id) => join(qr, "kanban", "tasks", id, "questions.yaml");
  const qlog = () => readFileSync(join(qr, "kanban", "log.jsonl"), "utf8").trim().split("\n").map((l) => JSON.parse(l))
    .filter((l) => l.change && "question" in l.change).map((l) => `${l.id}#${l.change.question} ${l.change.status}`);
  check("init a ledger for questions", run(["init", "--root", qr]).code === 0 && q(["add", "Export"]).out === "W-0001" && q(["add", "Import"]).out === "W-0002");
  const ask1 = q(["ask", "W-0001", "CSV only, or CSV and Excel?", "--kind", "business"]);
  check("ask prints <id>#<n>", ask1.out === "W-0001#1", ask1.out + ask1.err);
  check("ask - reads stdin", q(["ask", "W-0002", "-"], { input: "Which date format?\nISO or local.\n", env: { DECK_NOW: "2026-11-03T02:00:00Z" } }).out === "W-0002#1");
  check("ask refuses an unknown kind", q(["ask", "W-0001", "x", "--kind", "money"]).code === 1);
  check("ask refuses a kind in the wrong case", q(["ask", "W-0001", "x", "--kind", "Line"]).code === 1);
  const qs = q(["questions"], { env: { DECK_NOW: "2026-11-03T04:00:00Z" } }).out.split("\n");
  check("questions: oldest first, across tasks, with age, title and first line",
    qs.length === 2 && qs[0].startsWith("W-0001#1") && qs[0].includes("open") && qs[0].includes("3h") && qs[0].includes("Export") && qs[0].includes("CSV only")
    && qs[1].startsWith("W-0002#1") && qs[1].includes("Which date format?") && !qs[1].includes("ISO"), qs.join("\n"));
  check("received before answer is refused", q(["received", "W-0001#1"]).code === 1);
  check("answer", q(["answer", "W-0001#1", "-"], { input: "CSV only for now.\n", env: { HC_TERMINAL_ID: "t2" } }).out === "W-0001#1 answered");
  check("answer twice is refused", q(["answer", "W-0001#1", "again"]).code === 1);
  const qj = JSON.parse(q(["questions", "--json"]).out);
  check("questions --json", qj.length === 2 && qj[0].name === "W-0001#1" && qj[0].status === "answered" && qj[0].answer === "CSV only for now."
    && qj[0].answeredBy === "terminal:t2" && qj[0].title === "Export" && qj[0].kind === "business", JSON.stringify(qj));
  check("questions --open", q(["questions", "--open"]).out.split("\n").length === 1);
  const blocked = q(["done", "W-0001"]);
  check("done stops on a question not received", blocked.code === 1 && blocked.err.includes("W-0001#1 (answered)"), blocked.err);
  check("received", q(["received", "W-0001#1"]).out === "W-0001#1 received" && q(["received", "W-0001#1"]).code === 1);
  check("done once received", q(["done", "W-0001"]).code === 0);
  check("show counts the questions", q(["show", "W-0001"]).out.includes("questions: 0 open, 0 answered, 1 received"));
  check("show --json has them", JSON.parse(q(["show", "W-0001", "--json"]).out).questions[0].received === "2026-11-03T01:00:00Z");
  const forced = q(["done", "W-0002", "--force"]);
  const lastLog = readFileSync(join(qr, "kanban", "log.jsonl"), "utf8").trim().split("\n").map((l) => JSON.parse(l)).pop();
  check("done --force closes it and logs forced", forced.code === 0 && lastLog.id === "W-0002" && JSON.stringify(lastLog.change.forced) === "[1]", JSON.stringify(lastLog));
  check("the log has the numbers, not the text", qlog().join("|") === "W-0001#1 open|W-0002#1 open|W-0001#1 answered|W-0001#1 received"
    && !readFileSync(join(qr, "kanban", "log.jsonl"), "utf8").includes("CSV"), qlog().join("|"));
  writeFileSync(qfile("W-0001"), "");
  check("n is not reused after the file is emptied by hand", q(["ask", "W-0001", "Again?"]).out === "W-0001#2");
  const qasks = await Promise.all(Array.from({ length: 10 }, (_, i) => runAsync(["ask", "W-0002", `parallel ${i}`], { cwd: qr })));
  check("parallel asks get different numbers", qasks.every((x) => x.code === 0) && new Set(qasks.map((x) => x.out)).size === 10
    && qasks.map((x) => x.out).sort().join() === Array.from({ length: 10 }, (_, i) => `W-0002#${i + 2}`).sort().join(), JSON.stringify(qasks));
  check("lint ok with questions", q(["lint"]).code === 0, q(["lint"]).out);
  const askLine = q(["ask", "W-0002", "Change the gate?", "--kind", "line"]);
  check("ask accepts kind line", askLine.code === 0 && /^W-0002#\d+$/.test(askLine.out) && readFileSync(qfile("W-0002"), "utf8").includes("kind: line"), askLine.out + askLine.err);
  check("lint ok with kind line", q(["lint"]).code === 0, q(["lint"]).out);
  writeFileSync(qfile("W-0001"), "- n: 1\n  text: x\n  status: received\n  asked: \"2026-11-03T01:00:00Z\"\n");
  const qlint = q(["lint"]);
  check("lint names a question whose times don't fit", qlint.code === 1 && qlint.out.includes("W-0001#1: received but no answer"), qlint.out);

  // Routines: deck tick on a clock set by DECK_NOW, events from start/review/done/set, routine list/run, lint.
  const rr = join(yard, "projects", "r");
  mkdirSync(join(rr, "kanban", "routines"), { recursive: true });
  const at = (t) => ({ cwd: rr, env: { DECK_NOW: t } });
  const r = (args, t = "2026-11-02T08:00:00Z") => run(args, at(t));
  check("init a ledger for routines", run(["init", "--root", rr, "--repo", "github.com/x/app"]).code === 0);
  const routineFile = (name, body) => writeFileSync(join(rr, "kanban", "routines", `${name}.yaml`), body);
  routineFile("daily", "# kept\ntitle: Daily look\nbrief: |\n  Look.\nprofile: sonnet\nrepo: github.com/x/app\nschedule: \"0 9 * * *\"\n");
  const rtasks = () => readdirSync(join(rr, "kanban", "tasks")).sort();
  const rlog = () => readFileSync(join(rr, "kanban", "log.jsonl"), "utf8").trim().split("\n").filter(Boolean)
    .map((l) => JSON.parse(l)).filter((l) => "routine" in l).map((l) => `${l.routine} ${l.fired ?? "skipped " + l.skipped} ${l.by}`);
  const rstate = () => JSON.parse(readFileSync(join(rr, "kanban", "state", "routines.json"), "utf8"));
  check("first tick fires nothing and notes the time", r(["tick"]).code === 0 && rtasks().length === 0 && rstate().daily === "2026-11-02T08:00:00Z");
  check("tick before the time: nothing", r(["tick"], "2026-11-02T08:59:00Z").code === 0 && rtasks().length === 0);
  const t9 = r(["tick"], "2026-11-02T09:00:00Z");
  check("tick at the time adds one", t9.out === "daily: added W-0001" && rtasks().join() === "W-0001", t9.out + t9.err);
  check("tick again adds nothing", r(["tick"], "2026-11-02T09:00:00Z").out === "(nothing due)" && r(["tick"], "2026-11-02T09:05:00Z").code === 0 && rtasks().length === 1);
  const rw1 = JSON.parse(r(["show", "W-0001", "--json"]).out);
  check("the added task", rw1.task.title === "Daily look" && rw1.task.routine === "daily" && rw1.task.profile === "sonnet"
    && rw1.task.repo === "github.com/x/app" && rw1.task.status === "todo" && rw1.brief === "Look.\n", JSON.stringify(rw1));
  check("the routine file is only read", readFileSync(join(rr, "kanban", "routines", "daily.yaml"), "utf8").startsWith("# kept\n"));
  const skip = r(["tick"], "2026-11-03T09:00:00Z");
  check("open stops it while its task isn't done", skip.out === "daily: skipped (open)" && rtasks().length === 1, skip.out + skip.err);
  check("done", r(["done", "W-0001"], "2026-11-03T10:00:00Z").code === 0);
  check("after done, the next time adds again; missed days add one", r(["tick"], "2026-11-06T12:00:00Z").out === "daily: added W-0002"
    && r(["tick"], "2026-11-06T12:05:00Z").code === 0 && rtasks().length === 2);

  routineFile("after-done", "title: Look at what was done\non: [done]\n");
  routineFile("paused", "title: Paused\non: [done]\npaused: true\n");
  check("add a task", r(["add", "Work"]).out === "W-0003");
  check("start and review fire nothing", r(["start", "W-0003"]).code === 0 && r(["review", "W-0003"]).code === 0 && rtasks().length === 3);
  const dn = r(["done", "W-0003"]);
  check("done fires on: [done]", dn.out.includes("routine after-done: added W-0004") && rtasks().length === 4, dn.out + dn.err);
  check("done of its own task fires nothing new", r(["done", "W-0004"]).code === 0 && rtasks().length === 4);
  check("set status=done fires", r(["set", "W-0002", "status=done"]).out.includes("after-done: added W-0005"));
  check("run while open is skipped", r(["routine", "run", "after-done"]).out === "after-done: skipped (open)");
  check("run a paused routine is skipped", r(["routine", "run", "paused"]).out === "paused: skipped (paused)");
  check("run an unknown routine fails", r(["routine", "run", "nope"]).code === 1);
  const wantLog = ["daily W-0001 routine:daily", "daily skipped open routine:daily", "daily W-0002 routine:daily",
    "after-done W-0004 routine:after-done", "paused skipped paused routine:paused", // done W-0003 (routines in name order)
    "paused skipped paused routine:paused", // done W-0004: after-done added it, so only paused
    "after-done W-0005 routine:after-done", "paused skipped paused routine:paused", // set W-0002 status=done
    "after-done skipped open routine:after-done", "paused skipped paused routine:paused"]; // routine run
  check("the log has each firing and skip", rlog().join("|") === wantLog.join("|"), rlog().join("\n"));
  const rl = JSON.parse(r(["routine", "list", "--json"]).out || "[]");
  const daily = rl.find((x) => x.name === "daily");
  check("routine list", rl.length === 3 && daily.next === "2026-11-07T09:00:00Z" && daily.notDone === 0 && daily.open === 1, JSON.stringify(rl));

  routineFile("bad", "title: x\nschedule: \"0 25 * * *\"\nopen: 0\nrepo: github.com/x/nope\n");
  routineFile("none", "title: x\n");
  const rlint = r(["lint"]);
  check("lint names the routines' problems", rlint.code === 1 && ["routines/bad.yaml: schedule", "routines/bad.yaml: open", "routines/bad.yaml: repo",
    "routines/none.yaml: needs schedule or on"].every((p) => rlint.out.includes(p)), rlint.out);
  check("tick with a bad routine fails but fires the rest", r(["tick"], "2026-11-07T09:00:00Z").code === 1 && rtasks().length === 6);
  check("a DECK_NOW that isn't a time is refused", r(["tick"], "soon").code === 1);
  writeFileSync(join(rr, "kanban", "config.yaml"), "version: 3\n");
  const before = readFileSync(join(rr, "kanban", "state", "routines.json"), "utf8");
  check("routine commands refuse another version", [r(["tick"]), r(["routine", "list"]), r(["routine", "run", "daily"])].every((x) => x.code === 1 && x.err.includes("format")));
  check("and write nothing", readFileSync(join(rr, "kanban", "state", "routines.json"), "utf8") === before && rtasks().length === 6);

  const g = (args, cwd) => spawnSync("git", args, { cwd, encoding: "utf8" });
  const gitRepo = (dir) => {
    mkdirSync(join(dir, "src"), { recursive: true });
    g(["init", "-q"], dir);
    g(["-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"], dir);
  };
  if (g(["--version"], yard).status === 0) {
    // From the repository: the project whose repos list it. Entries that aren't exactly
    // host/owner/repo (a trailing or leading /, no host) match nothing.
    const app = ghq("app");
    gitRepo(app);
    for (const [slug, entry] of [["d", "github.com/x/app/"], ["e", "/github.com/x/app"], ["f", "x/app"]]) {
      mkdirSync(join(yard, "projects", slug, "kanban"), { recursive: true });
      writeFileSync(join(yard, "projects", slug, "kanban", "config.yaml"), `version: 2\nrepos: ["${entry}"]\n`);
    }
    const fromRepo = run(["root"], { cwd: join(app, "src") });
    check("root found from the repository (projects/a)", same(fromRepo.out, root), fromRepo.out + fromRepo.err);
    const added = run(["add", "In the repo", "--json"], { cwd: join(app, "src") });
    check("add in the repository fills repo", JSON.parse(added.out || "{}").repo === "github.com/x/app", added.out + added.err);
    const envWins = run(["root"], { cwd: app, env: { KANBAN_ROOT: rootB } });
    check("KANBAN_ROOT wins over the repository", same(envWins.out, rootB), envWins.out + envWins.err);

    // A worktree outside the yard still finds the project through its main checkout.
    const outside = mkdtempSync(join(tmpdir(), "deck-wt-"));
    const wt = join(outside, "wt");
    g(["worktree", "add", "-q", "-b", "work/3-after", wt], app);
    const fromWt = run(["root"], { cwd: wt });
    check("root found from a worktree outside it", same(fromWt.out, root), fromWt.out + fromWt.err);
    const s3 = JSON.parse(run(["start", "W-0003", "--json"], { cwd: wt }).out || "{}");
    check("start records the worktree's branch and repo", s3.branch === "work/3-after" && s3.repo === "github.com/x/app", JSON.stringify(s3));
    g(["worktree", "remove", "--force", wt], app);

    // On a yard, projects/<slug> may be a link to the repository (the ledger then sits in its
    // checkout, out of git). Finding the root and writing still work through that link.
    const lib = ghq("lib"), rootC = join(yard, "projects", "c");
    gitRepo(lib);
    let linked = true;
    try { symlinkSync(join("..", ".ghq", "github.com", "x", "lib"), rootC, "junction"); }
    catch (e) { linked = false; console.log(`skip: no link for projects/c (${e.code})`); }
    if (linked) {
      check("init in a linked project", run(["init", "--root", rootC, "--repo", "github.com/x/lib"]).code === 0);
      const wtC = join(outside, "wt-c");
      g(["worktree", "add", "-q", "-b", "work/c", wtC], lib);
      const fromC = run(["root"], { cwd: wtC });
      check("root found for a linked project", same(fromC.out, rootC), fromC.out + fromC.err);
      const addC = run(["add", "Linked work"], { cwd: wtC });
      const taskC = join(lib, "kanban", "tasks", "W-0001", "task.yaml");
      check("add through a linked project", addC.code === 0 && existsSync(taskC) && readFileSync(taskC, "utf8").includes("repo: github.com/x/lib"), addC.out + addC.err);
      const dirty = g(["status", "--porcelain", "--untracked-files=all"], lib).stdout.trim();
      check("the ledger stays out of git", dirty === "", dirty);
      g(["worktree", "remove", "--force", wtC], lib);
    }
    rmSync(outside, { recursive: true, force: true });

    // Two projects with the same repository: an error that names both.
    writeFileSync(join(rootB, "kanban", "config.yaml"), "version: 2\nrepos: [github.com/x/app]\n");
    const both = run(["root"], { cwd: app });
    check("two projects with one repository is an error naming both",
      both.code === 1 && both.err.includes("projects/a") && both.err.includes("projects/b"), both.out + both.err);
  } else console.log("skip: no git");
} finally {
  rmSync(yard, { recursive: true, force: true });
}

console.log(`${bin}: ${passed} passed, ${failed} failed`);
process.exitCode = failed ? 1 : 0;
