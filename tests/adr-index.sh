#!/usr/bin/env bash
# tests/adr-index.sh [--root <dir>]
#
# ADR の冒頭の決定の索引がずれていないこと（ADR-0033 決めたこと 5・6、受け入れ条件 1〜4）。
#   - 全部の ADR に索引（見出し「## 決定の索引」）が要る。索引が冒頭の最初の節にあり、表が 4 列（# ・何を決めたか・今の正・今）で、
#     番号の並びが「決めたこと」の番号の並びと同じ。「今の正」は「この ADR」か、repo に在るパスを `…` で書く。
#     「今」は 生きている・覆された・役目を終えた・まだ実装していない のどれかで始まる。
#     覆されたは、どこが覆したかを「覆された → ADR-NNNN」か「覆された → この ADR の N」で書く（あとに（…）を添えてよい）。
#     ADR-NNNN は在る別の ADR、N はその ADR の「決めたこと」に在る、行の自分と別の番号。
#   - 全部の ADR: 状態行（「- 状態: **決定…**」）があり、README の ADR の一覧に行があって、状態の語（決定・提案・覆された）が同じ。
#   - 全部の ADR に決定の記録が要る（AGENTS.md の表の決まり）。docs/decisions/ の「YYYY-MM-DD-adr-NNNN-<題>.md」か、
#     docs/decisions.md の凍結した表（§14）の行で、出典の欄が在る ADR のファイル（docs/adr/NNNN-….md）を同じ名前で指すもの。
#     §14 の外の行と、番号だけ合って在りえない名前を指すリンクは数えない。
# 引数なしは repo を見たあと、壊した写し 10 個（索引の 1 行を抜く・在りえないパスを書く・README の状態を変える・
# 索引の節を消す・覆したところを消す・在りえない ADR を覆した先に書く・記録の無い ADR を足す・§14 の行を消す・
# §14 の行のリンクを在りえない名前にする・docs/decisions/ のファイルを消す）でそれぞれ落ちることを確かめる。
# --root はその写しを見るときに使う。
set -euo pipefail
cd "$(dirname "$0")/.."

check() { # check <root>
  python3 - "$1" <<'PY'
import os, re, sys

root = sys.argv[1]
adr_dir = os.path.join(root, "docs", "adr")
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

files = sorted(f for f in os.listdir(adr_dir) if re.match(r"^[0-9]{4}-.+\.md$", f))
nums = {f[:4] for f in files}
states = {}
for f in files:
    path = f"docs/adr/{f}"
    with open(os.path.join(adr_dir, f), encoding="utf-8") as fh:
        lines = fh.read().split("\n")
    num = f[:4]
    if not lines[0].startswith(f"# ADR-{num}: "):
        fail(f"{path}: 1 行目が「# ADR-{num}: 」で始まらない")
    st = next((m.group(1) for ln in lines if (m := re.match(r"^- 状態: \*\*(.+?)\*\*", ln))), None)
    if st is None or status_word(st) is None:
        fail(f"{path}: 状態行（- 状態: **決定…** / **提案…** / **覆された…**）が無い")
    else:
        states[f] = status_word(st)

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

readme = {}
with open(os.path.join(root, "README.md"), encoding="utf-8") as fh:
    for ln in fh:
        m = re.match(r"^\s*- \[([0-9]{4}) [^\]]*\]\(docs/adr/([^)]+)\): (.+)$", ln.rstrip("\n"))
        if m:
            if not m.group(2).startswith(m.group(1) + "-"):
                fail(f"README.md: 一覧の番号 {m.group(1)} とファイル {m.group(2)} が違う")
            readme[m.group(2)] = m.group(3)
for f, st in states.items():
    if f not in readme:
        fail(f"README.md: ADR の一覧に docs/adr/{f} の行が無い")
    elif status_word(readme[f]) != st:
        fail(f"README.md: 一覧の docs/adr/{f} の状態「{readme[f]}」が ADR の状態行「{st}」と違う")
for f in readme:
    if f not in files:
        fail(f"README.md: 一覧の docs/adr/{f} が無い")

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
    print(f"ok   ADR {len(files)} 本の状態行と README の一覧が揃い、{len(files)} 本すべての索引が本文と repo に合い、決定の記録がある")
sys.exit(1 if fails else 0)
PY
}

case "${1:-}" in
  --root) check "$2"; exit ;;
  '') ;;
  *) sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2 ;;
esac

fail=0
check . || fail=1

# 自己確認（受け入れ条件 4）: 写しの根は docs/adr・docs/decisions・docs/decisions.md と README.md だけを写し、ほかは repo へのリンクにする（「今の正」のパスを本物で見る）
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
setup() {
  rm -rf "$tmp/r"; mkdir -p "$tmp/r/docs"
  for p in * .[!.]*; do [ -e "$p" ] && [ "$p" != docs ] && [ "$p" != README.md ] && [ "$p" != .git ] && ln -s "$PWD/$p" "$tmp/r/$p"; done
  for p in docs/*; do case "$p" in docs/adr | docs/decisions | docs/decisions.md) ;; *) ln -s "$PWD/$p" "$tmp/r/$p" ;; esac; done
  cp -r docs/adr docs/decisions docs/decisions.md "$tmp/r/docs/"; cp README.md "$tmp/r/README.md"
}
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
grep -q '^  - \[0003 .*: 決定$' README.md || { echo "FAIL 自己確認の前提: README の一覧の 0003 が「決定」でない"; exit 1; }
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
setup; sed -i 's/^\(  - \[0003 .*\): 決定$/\1: 提案/' "$tmp/r/README.md"; expect fail 'README の状態を変えた写しで落ちる' '状態「提案」が ADR の状態行「決定」と違う'

setup; sed -i '/^## 決定の索引$/,/^## /{/^## 決定の索引$/d;/^## /!d}' "$tmp/r/$a33"; expect fail '索引の節を消した写しで落ちる' "$a33: 決定の索引（見出し「## 決定の索引」）が無い"
setup; sed -i 's/| 覆された → この ADR の 22 |$/| 覆された。 |/' "$tmp/r/$a02"; expect fail '覆したところを消した写しで落ちる' '索引の 21 の「今」が覆されたのに、どこが覆したかを'
setup; sed -i '0,/| 覆された → ADR-0003（/s//| 覆された → ADR-9999（/' "$tmp/r/$a02"; expect fail '在りえない ADR を覆した先に書いた写しで落ちる' '索引の 1 を覆した ADR-9999 が、在る別の ADR でない'

# 記録の無い ADR を足す: 0041 を 9998 に写して題の行と README の一覧の行を足す（索引と状態行は 0041 のまま正しい）
setup; sed '1s/^# ADR-0041: /# ADR-9998: /' "$a41" >"$tmp/r/docs/adr/9998-no-record.md"
sed -i '/^  - \[0041 /a\  - [9998 記録の無い ADR](docs/adr/9998-no-record.md): 決定' "$tmp/r/README.md"
expect fail '決定の記録の無い ADR を足した写しで落ちる' 'docs/adr/9998-no-record.md: 決定の記録'
setup; sed -i '/^## 14\. /,/^## 15\. /{/](adr\/0002-/d}' "$tmp/r/docs/decisions.md"; expect fail 'decisions.md §14 の行を消した写しで落ちる（§14 の外の行は数えない）' 'docs/adr/0002-herdr-spaces.md: 決定の記録'
setup; sed -i '/^## 14\. /,/^## 15\. /s/](adr\/0002-herdr-spaces\.md)/](adr\/0002-no-such-title.md)/' "$tmp/r/docs/decisions.md"
expect fail 'decisions.md §14 の行のリンクを番号だけ合う在りえない名前にした写しで落ちる' 'docs/adr/0002-herdr-spaces.md: 決定の記録'
setup; rm "$tmp/r/$d41"; expect fail 'docs/decisions/ の記録を消した写しで落ちる' "$a41: 決定の記録"

exit "$fail"
