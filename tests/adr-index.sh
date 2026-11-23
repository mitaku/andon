#!/usr/bin/env bash
# tests/adr-index.sh [--root <dir>] [--list | --write-list]
#
#   --list        今の ADR の一覧を、1 本 1 行で番号の順に出す（ADR-0055 決めたこと 3）。
#   --write-list  docs/adr/README.md の見出し「## 一覧」から下を、CHANGELOG.md の一番上のバージョンと本数の 1 行と、--list の出力で
#                 置き換える。打つのはバージョンを切る PR（docs/operate.md §14）。ADR の PR は一覧に触らない。
#   --root <dir>  その木を見る・読む・書く（自己確認の写しに使う）。
#
# どちらも無ければ検査する。ADR の冒頭の決定の索引がずれていないこと（ADR-0033 決めたこと 5・6、受け入れ条件 1〜4）。
#   - 全部の ADR に索引（見出し「## 決定の索引」）が要る。索引が冒頭の最初の節にあり、表が 4 列（# ・何を決めたか・今の正・今）で、
#     番号の並びが「決めたこと」の番号の並びと同じ。「今の正」は「この ADR」か、repo に在るパスを `…` で書く。
#     「今」は 生きている・覆された・役目を終えた・まだ実装していない のどれかで始まる。
#     覆されたは、どこが覆したかを「覆された → ADR-NNNN」か「覆された → この ADR の N」で書く（あとに（…）を添えてよい）。
#     ADR-NNNN は在る別の ADR、N はその ADR の「決めたこと」に在る、行の自分と別の番号。
#   - 全部の ADR: 状態行（「- 状態: **決定…**」。語は 決定・提案・覆された）がある。
#   - docs/adr/README.md に見出し「## 一覧」があり、その下の 1 行のバージョンが CHANGELOG.md の一番上のバージョンと同じで、
#     続く行はどれも一覧の行の形で、在る ADR を同じ番号で指す（ADR-0055 決めたこと 5）。一覧は前のバージョンまでの写しなので、
#     ADR ごとの行があることと、行のタイトル・状態が今の ADR と同じことは見ない。
#   - 全部の ADR に決定の記録が要る（AGENTS.md の表の決まり）。docs/decisions/ の「YYYY-MM-DD-adr-NNNN-<題>.md」か、
#     docs/decisions.md の凍結した表（§14）の行で、出典の欄が在る ADR のファイル（docs/adr/NNNN-….md）を同じ名前で指すもの。
#     §14 の外の行と、番号だけ合って在りえない名前を指すリンクは数えない。
# 引数なしは repo を見たあと、壊した写し（索引・状態行・決定の記録・一覧）でそれぞれ落ちることと、ADR を足しただけ・状態を
# 変えただけの写しで落ちないこと、--list と --write-list が出すものを確かめる。
set -euo pipefail
caller=$PWD # 相対の --root は打った所から読む
cd "$(dirname "$0")/.."

adr() { # adr check|list|write <root>
  python3 - "$1" "$2" <<'PY'
import os, re, sys

mode, root = sys.argv[1:]
adr_dir = os.path.join(root, "docs", "adr")
LIST = "docs/adr/README.md"
LIST_HEADING = "## 一覧"
LIST_ROW = re.compile(r"- \[([0-9]{4}) .+\]\(([^()\s]+)\): (?:決定|提案|覆された)")
LIST_TOP = re.compile(r"CHANGELOG の一番上が (v[0-9]+\.[0-9]+\.[0-9]+) のときに作り直した [0-9]+ 本。.*")
VERSION = re.compile(r"##\s+(v[0-9]+\.[0-9]+\.[0-9]+)(?![0-9.])")
REWRITE = "bash tests/adr-index.sh --write-list を打つ"
# タイトルから落とす末尾の括弧（「（仕様・設計。日付…）」「（日付…）」）。ほかの括弧はタイトルのうち
TITLE_TAIL = re.compile(r"（(?:仕様・設計|[0-9]{4}-[0-9]{2}-[0-9]{2})(?:[^（）]|（[^（）]*）)*）$")
NOW = ("生きている", "覆された", "役目を終えた", "まだ実装していない")
HEADER = ["#", "何を決めたか", "今の正", "今"]
OVERRIDDEN = re.compile(r"覆された → (?:ADR-([0-9]{4})|この ADR の ([0-9]+))(?:（[^（）]+(?:（[^（）]*）[^（）]*)*）)?")
fails = []

def fail(msg):
    fails.append(msg)

def sections(lines):
    """「## 」の見出しごとに (見出し, 行の並び) を返す。"""
    out, cur = [], None
    for ln in lines:
        if ln.startswith("## "):
            cur = (ln[3:].strip(), [])
            out.append(cur)
        elif cur is not None:
            cur[1].append(ln)
    return out

def status_word(text):
    m = re.match(r"(決定|提案|覆された)", text.strip())
    return m.group(1) if m else None

def read(rel):
    """root からの相対パスの行の並び。読めなければ落として None。"""
    try:
        with open(os.path.join(root, rel), encoding="utf-8") as fh:
            return fh.read().split("\n")
    except OSError as e:
        fail(f"{rel} を読めない（{e.strerror}）")
        return None

def stop():
    for m in fails:
        print(f"FAIL {m}", file=sys.stderr)
    sys.exit(1)

files = sorted(f for f in os.listdir(adr_dir) if re.match(r"^[0-9]{4}-.+\.md$", f))
nums = {f[:4] for f in files}

# 1 行目と状態行は、検査と一覧が同じ所で読む
bodies, listed = {}, []
for f in files:
    path = f"docs/adr/{f}"
    lines = read(path)
    if lines is None:
        continue
    bodies[f] = lines
    head = f"# ADR-{f[:4]}: "
    if not lines[0].startswith(head):
        fail(f"{path}: 1 行目が「{head}」で始まらない")
    st = next((m.group(1) for ln in lines if (m := re.match(r"^- 状態: \*\*(.+?)\*\*", ln))), None)
    word = status_word(st) if st is not None else None
    if word is None:
        fail(f"{path}: 状態行（- 状態: **決定…** / **提案…** / **覆された…**）が無い")
    title = TITLE_TAIL.sub("", lines[0][len(head):].rstrip()).rstrip()
    listed.append(f"- [{f[:4]} {title}]({f}): {word}")

if mode == "list":
    if fails:
        stop()
    print("\n".join(listed))
    sys.exit()

top = None  # CHANGELOG.md の一番上のバージョン
changelog = read("CHANGELOG.md")
if changelog is not None:
    top = next((m.group(1) for ln in changelog if (m := VERSION.match(ln))), None)
    if top is None:
        fail("CHANGELOG.md にバージョンの見出し（## vX.Y.Z）が無い")

board = read(LIST)
if board is not None and LIST_HEADING not in board:
    fail(f"{LIST}: 見出し「{LIST_HEADING}」が無い")
    board = None

if mode == "write":
    if fails:
        stop()
    summary = f"CHANGELOG の一番上が {top} のときに作り直した {len(files)} 本。それより新しい ADR と変わった状態は `bash tests/adr-index.sh --list` で出せる。"
    out = board[:board.index(LIST_HEADING) + 1] + ["", summary, ""] + listed
    dst = os.path.join(root, LIST)
    part = f"{dst}.{os.getpid()}.tmp"  # 同時に打った回と一時ファイルを取り合わない
    with open(part, "w", encoding="utf-8") as fh:
        fh.write("\n".join(out) + "\n")
    os.replace(part, dst)
    sys.exit()

for f, lines in bodies.items():
    path = f"docs/adr/{f}"
    num = f[:4]
    secs = sections(lines)
    idx = [s for s in secs if s[0] == "決定の索引"]
    if not idx:
        fail(f"{path}: 決定の索引（見出し「## 決定の索引」）が無い")
        continue
    dec = next((s for s in secs if s[0].startswith("決めたこと")), None)
    want = [int(m.group(1)) for ln in (dec[1] if dec else []) if (m := re.match(r"^([0-9]+)\. ", ln))]
    if secs[0][0] != "決定の索引":
        fail(f"{path}: 決定の索引が冒頭（状態行の下の最初の節）に無い（最初の節は「{secs[0][0]}」）")
    rows = [ln for ln in idx[0][1] if ln.startswith("|")]
    cells = [[c.strip() for c in r.strip().strip("|").split("|")] for r in rows]
    if len(cells) < 2 or cells[0] != HEADER or not all(re.fullmatch(r":?-+:?", c) for c in cells[1]):
        fail(f"{path}: 索引の表の見出しが「| # | 何を決めたか | 今の正 | 今 |」と区切りの行でない")
        continue
    got = []
    for c in cells[2:]:
        if len(c) != 4:
            fail(f"{path}: 索引の行が 4 列でない: | {' | '.join(c)} |")
            continue
        n, what, src, now = c
        if not n.isdigit():
            fail(f"{path}: 索引の # が番号でない: {n}")
            continue
        got.append(int(n))
        if not what:
            fail(f"{path}: 索引の {n} の「何を決めたか」が空")
        if src != "この ADR":
            ps = re.findall(r"`([^`]+)`", src)
            if not ps:
                fail(f"{path}: 索引の {n} の「今の正」が「この ADR」でも `パス` でもない: {src}")
            for p in ps:
                if p.startswith("/") or ".." in p.split("/") or not os.path.exists(os.path.join(root, p)):
                    fail(f"{path}: 索引の {n} の「今の正」のパスが repo に無い: {p}")
        w = next((w for w in NOW if now.startswith(w)), None)
        if w is None:
            fail(f"{path}: 索引の {n} の「今」が {'・'.join(NOW)} のどれでもない: {now}")
        elif w == "覆された":
            m = OVERRIDDEN.fullmatch(now)
            if m is None:
                fail(f"{path}: 索引の {n} の「今」が覆されたのに、どこが覆したかを「覆された → ADR-NNNN」か「覆された → この ADR の N」で書いていない: {now}")
            elif m.group(1) and (m.group(1) == num or m.group(1) not in nums):
                fail(f"{path}: 索引の {n} を覆した ADR-{m.group(1)} が、在る別の ADR でない")
            elif m.group(2) and (int(m.group(2)) == int(n) or int(m.group(2)) not in want):
                fail(f"{path}: 索引の {n} を覆した「この ADR の {m.group(2)}」が、この ADR の「決めたこと」に在る別の番号でない")
    if got != want:
        fail(f"{path}: 索引の番号 {got} が「決めたこと」の番号 {want} と違う")

# 一覧は前のバージョンまでの写し。見出しの下の 1 行のバージョンと、行が指す先だけを見る
if board is not None:
    below = [ln for ln in board[board.index(LIST_HEADING) + 1:] if ln.strip()]
    m = LIST_TOP.fullmatch(below[0]) if below else None
    if m is None:
        fail(f"{LIST}: 見出し「{LIST_HEADING}」の下の 1 行（どのバージョンで作り直した何本か）が読めない。{REWRITE}")
    elif top is not None and m.group(1) != top:
        fail(f"{LIST}: 一覧を作り直したバージョン {m.group(1)} が CHANGELOG.md の一番上の {top} と違う。{REWRITE}")
    for ln in below[1:]:
        row = LIST_ROW.fullmatch(ln)
        if row is None:
            fail(f"{LIST}: 一覧の行の形（- [NNNN タイトル](NNNN-<名前>.md): 状態の語）でない: {ln}")
            continue
        num, name = row.groups()
        if not name.startswith(num + "-"):
            fail(f"{LIST}: 一覧の番号 {num} とファイル {name} が違う")
        if name not in files:
            fail(f"{LIST}: 一覧の docs/adr/{name} が無い")

# 決定の記録: ADR 1 本につき docs/decisions/ の YYYY-MM-DD-adr-NNNN-<題>.md か、docs/decisions.md の凍結した表（§14）の行
recorded = {}
dec_dir = os.path.join(root, "docs", "decisions")
for d in sorted(os.listdir(dec_dir)) if os.path.isdir(dec_dir) else []:
    m = re.match(r"^[0-9]{4}-[0-9]{2}-[0-9]{2}-adr-([0-9]{4})-.+\.md$", d)
    if m:
        recorded.setdefault(m.group(1), f"docs/decisions/{d}")
dlog = os.path.join(root, "docs", "decisions.md")
if os.path.exists(dlog):
    with open(dlog, encoding="utf-8") as fh:
        frozen = next((s for s in sections(fh.read().split("\n")) if s[0].startswith("14. ")), None)
    if frozen is None:
        fail("docs/decisions.md: 凍結した ADR の表（見出し「## 14. 」）が無い")
    else:
        for ln in frozen[1]:
            if not ln.startswith("| 20"):
                continue
            # リンク先が在る ADR のファイルと同じ名前のときだけ数える（番号だけ合う在りえない名前は数えない）
            for name in re.findall(r"\]\(adr/([0-9]{4}-[^)]*\.md)\)", ln.rstrip().rstrip("|").rsplit("|", 1)[-1]):
                if name in files:
                    recorded.setdefault(name[:4], "docs/decisions.md §14")
else:
    fail("docs/decisions.md が無い")
for f in files:
    if f[:4] not in recorded:
        fail(f"docs/adr/{f}: 決定の記録（docs/decisions/YYYY-MM-DD-adr-{f[:4]}-<題>.md か docs/decisions.md §14 の行）が無い")

for m in fails:
    print(f"FAIL {m}")
if not fails:
    print(f"ok   ADR {len(files)} 本すべての状態行があり、索引が本文と repo に合い、決定の記録がある。一覧の見出しは CHANGELOG の {top} に合い、行の指す先が在る")
sys.exit(1 if fails else 0)
PY
}
check() { adr check "$1"; }
usage() { sed -n '2,/^set /{/^set /d;s/^# \{0,1\}//;p;}' "$0" >&2; exit 2; }

root=
if [ "${1:-}" = --root ]; then
  [ -n "${2:-}" ] || usage
  case "$2" in /*) root=$2 ;; *) root=$caller/$2 ;; esac
  shift 2
fi
[ "$#" -le 1 ] || usage
case "${1:-}" in
  --list) adr list "${root:-.}"; exit ;;
  --write-list) adr write "${root:-.}"; exit ;;
  '') if [ -n "$root" ]; then check "$root"; exit; fi ;;
  *) usage ;;
esac

fail=0
check . || fail=1

# 自己確認（ADR-0033 受け入れ条件 4、ADR-0055 受け入れ条件 2・3）: 写しの根は docs/adr・docs/decisions・docs/decisions.md と CHANGELOG.md だけを
# 写し、ほかは repo へのリンクにする（「今の正」のパスを本物で見る。--write-list は写しの docs/adr/README.md だけに書く）
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
setup() {
  rm -rf "$tmp/r"; mkdir -p "$tmp/r/docs"
  for p in * .[!.]*; do [ -e "$p" ] && [ "$p" != docs ] && [ "$p" != CHANGELOG.md ] && [ "$p" != .git ] && ln -s "$PWD/$p" "$tmp/r/$p"; done
  for p in docs/*; do case "$p" in docs/adr | docs/decisions | docs/decisions.md) ;; *) ln -s "$PWD/$p" "$tmp/r/$p" ;; esac; done
  cp -r docs/adr docs/decisions docs/decisions.md "$tmp/r/docs/"; cp CHANGELOG.md "$tmp/r/CHANGELOG.md"
}
verify() { # verify <説明> <command>…
  local what=$1; shift
  if "$@"; then echo "ok   自己確認: $what"; else echo "FAIL 自己確認: $what"; fail=1; fi
}
copy() { bash tests/adr-index.sh --root "$tmp/r" "$@"; } # 写しに command を打つ
rejects() { ! "$@" >/dev/null 2>&1; }
expect() { # expect ok|fail <説明> [<落ちた文に要る語>]
  local out got
  if out=$(check "$tmp/r" 2>&1); then got=ok; else got=fail; fi
  if [ "$got" = "$1" ] && { [ -z "${3:-}" ] || grep -qF -- "$3" <<<"$out"; }; then
    echo "ok   自己確認: $2"
    if [ "$got" = fail ]; then sed -n 's/^FAIL /       落ちた文: /p' <<<"$out"; fi
  else
    echo "FAIL 自己確認: $2（$got）"; sed 's/^/       /' <<<"$out"; fail=1
  fi
}
a33=docs/adr/0033-adr-inventory-and-slimming.md
a15=docs/adr/0015-inbox-needs-you.md
a02=docs/adr/0002-herdr-spaces.md
grep -q '^| 3 | ' "$a33" || { echo "FAIL 自己確認の前提: $a33 の索引に 3 の行が無い"; exit 1; }
grep -q '`package/host/inbox/main.js`' "$a15" || { echo "FAIL 自己確認の前提: $a15 の索引に main.js のパスが無い"; exit 1; }
grep -q '^| 21 | .* | 覆された → この ADR の 22 |$' "$a02" || { echo "FAIL 自己確認の前提: $a02 の索引の 21 が「覆された → この ADR の 22」でない"; exit 1; }
grep -q '^| 1 | .* | 覆された → ADR-0003（' "$a02" || { echo "FAIL 自己確認の前提: $a02 の索引の 1 が「覆された → ADR-0003」でない"; exit 1; }
a03=docs/adr/0003-task-spaces.md
board=docs/adr/README.md
grep -q '^- 状態: \*\*決定' "$a03" || { echo "FAIL 自己確認の前提: $a03 の状態行が「決定」でない"; exit 1; }
grep -qF '](0002-herdr-spaces.md): ' "$board" || { echo "FAIL 自己確認の前提: $board の一覧に 0002 の行が無い"; exit 1; }
grep -qx '## 一覧' "$board" || { echo "FAIL 自己確認の前提: $board に見出し「## 一覧」が無い"; exit 1; }
cur=$(sed -n 's/^## \(v[0-9.]*\).*/\1/p' CHANGELOG.md | sed -n 1p)
[ -n "$cur" ] || { echo "FAIL 自己確認の前提: CHANGELOG.md にバージョンの見出しが無い"; exit 1; }
a41=docs/adr/0041-delegation-cap-out-of-command.md
d41=$(ls docs/decisions/*-adr-0041-*.md 2>/dev/null | head -1)
[ -n "$d41" ] || { echo "FAIL 自己確認の前提: docs/decisions/ に ADR-0041 の記録が無い"; exit 1; }
ls docs/decisions/*-adr-0002-*.md >/dev/null 2>&1 && { echo "FAIL 自己確認の前提: ADR-0002 の記録が docs/decisions/ にもある（§14 の行だけのはず）"; exit 1; }
[ "$(sed -n '/^## 14\. /,/^## 15\. /p' docs/decisions.md | grep -cF '](adr/0002-herdr-spaces.md)')" -eq 1 ] || { echo "FAIL 自己確認の前提: decisions.md §14 に ADR-0002 の行が 1 つでない"; exit 1; }
[ "$(grep -c '](adr/0002-' docs/decisions.md)" -gt 1 ] || { echo "FAIL 自己確認の前提: decisions.md の §14 の外に ADR-0002 を指す行が無い"; exit 1; }
[ ! -e docs/adr/9998-no-record.md ] || { echo "FAIL 自己確認の前提: docs/adr/9998-no-record.md が在る"; exit 1; }

setup; expect ok '壊していない写しは通る'
setup; sed -i '/^| 3 | /d' "$tmp/r/$a33"; expect fail '索引から 1 行を抜いた写しで落ちる' '索引の番号 [1, 2, 4, 5, 6, 7] が「決めたこと」の番号 [1, 2, 3, 4, 5, 6, 7] と違う'
setup; sed -i '0,/`package\/host\/inbox\/main.js`/s//`package\/host\/inbox\/no-such-file.js`/' "$tmp/r/$a15"; expect fail '在りえないパスを書いた写しで落ちる' '「今の正」のパスが repo に無い: package/host/inbox/no-such-file.js'
setup; sed -i '/^- 状態: /d' "$tmp/r/$a03"; expect fail '状態行を消した写しで落ちる' "$a03: 状態行"

setup; sed -i '/^## 決定の索引$/,/^## /{/^## 決定の索引$/d;/^## /!d}' "$tmp/r/$a33"; expect fail '索引の節を消した写しで落ちる' "$a33: 決定の索引（見出し「## 決定の索引」）が無い"
setup; sed -i 's/| 覆された → この ADR の 22 |$/| 覆された。 |/' "$tmp/r/$a02"; expect fail '覆したところを消した写しで落ちる' '索引の 21 の「今」が覆されたのに、どこが覆したかを'
setup; sed -i '0,/| 覆された → ADR-0003（/s//| 覆された → ADR-9999（/' "$tmp/r/$a02"; expect fail '在りえない ADR を覆した先に書いた写しで落ちる' '索引の 1 を覆した ADR-9999 が、在る別の ADR でない'

# ADR を足す: 0041 を 9998 に写して 1 行目の番号を替える（索引と状態行は 0041 のまま正しい）。記録が無ければ落ち、
# 記録を足せば、docs/adr/README.md にも README.md にも触らずに通る（ADR-0055 受け入れ条件 2）
setup; sed '1s/^# ADR-0041: /# ADR-9998: /' "$a41" >"$tmp/r/docs/adr/9998-no-record.md"
expect fail '決定の記録の無い ADR を足した写しで落ちる' 'docs/adr/9998-no-record.md: 決定の記録'
echo '# 足した ADR の決定の記録' >"$tmp/r/docs/decisions/2026-01-01-adr-9998-no-record.md"
expect ok 'ADR と決定の記録だけを足し、一覧に触らない写しは通る'
verify 'その写しの docs/adr/README.md は元のまま' cmp -s "$board" "$tmp/r/$board"
verify 'その写しの README.md は元のまま' cmp -s README.md "$tmp/r/README.md"
setup; sed -i '/^## 14\. /,/^## 15\. /{/](adr\/0002-/d}' "$tmp/r/docs/decisions.md"; expect fail 'decisions.md §14 の行を消した写しで落ちる（§14 の外の行は数えない）' 'docs/adr/0002-herdr-spaces.md: 決定の記録'
setup; sed -i '/^## 14\. /,/^## 15\. /s/](adr\/0002-herdr-spaces\.md)/](adr\/0002-no-such-title.md)/' "$tmp/r/docs/decisions.md"
expect fail 'decisions.md §14 の行のリンクを番号だけ合う在りえない名前にした写しで落ちる' 'docs/adr/0002-herdr-spaces.md: 決定の記録'
setup; rm "$tmp/r/$d41"; expect fail 'docs/decisions/ の記録を消した写しで落ちる' "$a41: 決定の記録"

# 一覧の作り直し忘れ（ADR-0055 受け入れ条件 3）。見出しの書き方の揺れと、今のバージョンで始まる別のバージョンでも落ちる
for h in '## v999.0.0 — 2026-01-01' '## v999.0.0' '##  v999.0.0 — 2026-01-01' '## v999.0.0—2026-01-01' "## ${cur}0 — 2026-01-01"; do
  setup; sed -i "0,/^## v/s//$h\n\n&/" "$tmp/r/CHANGELOG.md"
  expect fail "CHANGELOG の一番上に「$h」を足した写しで落ちる" '--write-list'
done
setup; sed -i '0,/^## /s//## v999.0.0 — 2026-01-01\n\n&/' "$tmp/r/CHANGELOG.md"
expect fail 'CHANGELOG の「未リリース」より上に新しいバージョンを足した写しで落ちる' '--write-list'
verify 'その写しに --write-list を打てる' copy --write-list
expect ok 'バージョンを足した写しは --write-list のあと通る'
verify '--write-list は見出しの下にそのバージョンを書く' grep -qF 'CHANGELOG の一番上が v999.0.0 のときに' "$tmp/r/$board"
setup; sed -i '/^## v/d' "$tmp/r/CHANGELOG.md"; expect fail 'CHANGELOG にバージョンの見出しが無い写しで落ちる' 'バージョンの見出し（## vX.Y.Z）が無い'
setup; rm "$tmp/r/CHANGELOG.md"; expect fail 'CHANGELOG の無い写しで落ちる' 'CHANGELOG.md を読めない'

# 一覧の形と、在りえない行
setup; sed -i 's/](0002-herdr-spaces\.md): /](0002-no-such-title.md): /' "$tmp/r/$board"
expect fail '一覧の行のファイルの名前を在りえない名前にした写しで落ちる' '一覧の docs/adr/0002-no-such-title.md が無い'
for row in '- [9997 在りえない番号](9997-no-such.md): 決定' '- [0001 番号違い](0002-herdr-spaces.md): 決定' \
  '- [0002 前の置き場からの相対](docs/adr/0002-herdr-spaces.md): 決定' '- [0002 範囲の外](../design.md): 決定' \
  "- [0002 引用符]('0002-herdr-spaces.md'): 決定" '  - [0002 字下げ](0002-herdr-spaces.md): 決定' \
  '-  [0002 空白が 2 つ](0002-herdr-spaces.md): 決定' '-[0002 空白が無い](0002-herdr-spaces.md): 決定' \
  '- [0002 状態の語でない](0002-herdr-spaces.md): 検討中' '手で足した文'; do
  setup; printf '%s\n' "$row" >>"$tmp/r/$board"; expect fail "一覧に「$row」を足した写しで落ちる" "$board: 一覧の"
done
setup; sed -i '/^## 一覧$/d' "$tmp/r/$board"; expect fail '見出し「## 一覧」を消した写しで落ちる' '見出し「## 一覧」が無い'
verify '見出しの無い写しに --write-list は書かない' rejects copy --write-list
for h in '##  一覧' '##一覧' '### 一覧'; do
  setup; sed -i "s/^## 一覧$/$h/" "$tmp/r/$board"; expect fail "見出しを「$h」にした写しで落ちる" '見出し「## 一覧」が無い'
done
setup; sed -i '/^CHANGELOG の一番上が /d' "$tmp/r/$board"; expect fail '見出しの下の 1 行を消した写しで落ちる' '--write-list'
setup; sed -i '/^## 一覧$/q' "$tmp/r/$board"; expect fail '見出しの下が空の写しで落ちる' '--write-list'
setup; rm "$tmp/r/$board"; expect fail 'docs/adr/README.md の無い写しで落ちる' "$board を読めない"

# 状態が変わっただけでは落ちない。--list は今の状態を出す
setup; copy --list >"$tmp/list.before"
verify '--list の行数が ADR の本数と同じ' test "$(wc -l <"$tmp/list.before")" -eq "$(ls docs/adr/[0-9]*.md | wc -l)"
verify '--list は番号の順' env LC_ALL=C sort -c "$tmp/list.before"
verify '--list の 0003 は「決定」' grep -q '^- \[0003 .*](0003-task-spaces\.md): 決定$' "$tmp/list.before"
sed -i '0,/^- 状態: \*\*決定/s//- 状態: **提案/' "$tmp/r/$a03"
expect ok '状態行を「提案」に変えただけの写しは通る'
copy --list >"$tmp/list.after"
verify 'その写しの --list は 0003 が「提案」に変わる' grep -q '^- \[0003 .*](0003-task-spaces\.md): 提案$' "$tmp/list.after"

# タイトルは末尾の「（仕様・設計。日付…）」だけを落とす
setup; sed -i '1s/.*/# ADR-0003: 分ける（`x ack`）（仕様・設計。2026-01-01（2 日目））/' "$tmp/r/$a03"; copy --list >"$tmp/list.after"
verify '--list のタイトルは末尾の括弧だけを落とす' grep -qF -- '- [0003 分ける（`x ack`）](0003-task-spaces.md): ' "$tmp/list.after"

# --write-list は見出しから上を残し、下を --list の出力にする。2 度打っても同じ。読めない ADR があれば書かない
setup; copy --write-list; cp "$tmp/r/$board" "$tmp/board"; copy --write-list
verify '--write-list は 2 度打っても同じ' cmp -s "$tmp/board" "$tmp/r/$board"
verify '--write-list は見出しから上を残す' cmp -s <(sed '/^## 一覧$/q' "$board") <(sed '/^## 一覧$/q' "$tmp/r/$board")
verify '--write-list の行は --list の出力と同じ' cmp -s <(copy --list) <(sed -n '/^## 一覧$/,$p' "$tmp/r/$board" | grep '^- ')
setup; sed -i '/^- 状態: /d' "$tmp/r/$a03"
verify '状態行の無い ADR があると --list は落ちる' rejects copy --list
verify '状態行の無い ADR があると --write-list は落ちる' rejects copy --write-list
verify '落ちた --write-list は一覧に書かない' cmp -s "$board" "$tmp/r/$board"

# --root の相対パスは打った所から読む（repo の根から読むと、写しでなく repo の一覧に書く）
setup; sed -i '0,/^## v/s//## v999.0.0\n\n&/' "$tmp/r/CHANGELOG.md"; cp "$board" "$tmp/board"
(cd "$tmp" && bash "$OLDPWD/tests/adr-index.sh" --root r --write-list) >/dev/null 2>&1 || true
verify '打った所からの相対の --root に --write-list を打つと、写しの一覧に書く' grep -qF 'CHANGELOG の一番上が v999.0.0 のときに' "$tmp/r/$board"
verify 'そのとき repo の一覧は元のまま' cmp -s "$tmp/board" "$board"

exit "$fail"
