#!/usr/bin/env bash
# tests/agents-md.sh [--root <dir>]
#
# repo の直下の AGENTS.md が「決まり｜守らせるもの」の対の表になっていること（ADR-0035 決めたこと 6、受け入れ条件 10）。
#   - 書いてよい行は、1 行目の「# 」の題・「## 」の見出し・空行・表の行だけ。表の外に決まり（箇条書きや段落）を書くと落ちる。
#   - 表はどれも、見出しが「| 決まり | 守らせるもの |」で、次の行が区切り。行は 2 列で、どちらも空でない（セルの中の | は \| と書く）。
#   - 守らせるものは「無し。」と理由か、`…` で名指した検査・command・hook・repo の設定。名指しの中のパスに見える語
#     （/ を含むか、.sh・.yml などで終わる語。<…>・*・...・$ を含む語と - で始まる語は除く）は、repo に在ること。
# 引数なしは repo を見たあと、壊した写し 4 つ（表の外に箇条書きを足す・守らせるものを空にする・在りえない検査を名指す・
# 「無し。」の理由を消す）でそれぞれ落ちることを確かめる。--root はその写しを見るときに使う。
# AGENTS.md だけを変えた PR でも走るよう、tests/lib/scope.sh の「いつも」に入れている。
set -euo pipefail
cd "$(dirname "$0")/.."

check() { # check <root>
  python3 - "$1" <<'PY'
import os, re, sys

root = sys.argv[1]
path = "AGENTS.md"
HEADER = ["決まり", "守らせるもの"]
EXT = re.compile(r"\.(sh|ya?ml|json|toml|go|md|py|mjs|tla|cfg|txt)$")
fails = []

def fail(n, msg):
    fails.append(f"{path}:{n}: {msg}")

def cells(line):
    """表の行をセルに割る（\\| は割らずに | に戻す）。"""
    body = line.strip()
    body = body[1:] if body.startswith("|") else body
    body = body[:-1] if body.endswith("|") and not body.endswith("\\|") else body
    return [c.strip().replace("\\|", "|") for c in re.split(r"(?<!\\)\|", body)]

def named(text):
    """守らせるものの `…` の中から、repo に在るはずのパスの語を返す。"""
    out = []
    for span in re.findall(r"`([^`]+)`", text):
        for w in span.split():
            if w.startswith("-") or any(s in w for s in ("<", ">", "*", "...", "$")):
                continue
            if "/" in w or EXT.search(w):
                out.append(w)
    return out

with open(os.path.join(root, path), encoding="utf-8") as fh:
    lines = fh.read().split("\n")

if not lines[0].startswith("# "):
    fail(1, "1 行目が「# 」の題でない")
rows = nones = 0
state = "out"  # out（表の外）・sep（見出しの次は区切り）・body（表の中）
for i, ln in enumerate(lines[1:], start=2):
    if not ln.startswith("|"):
        state = "out"
        if ln.strip() == "" or ln.startswith("## "):
            continue
        fail(i, f"表の外に書いた行（決まりは「| 決まり | 守らせるもの |」の表の行で書く）: {ln}")
        continue
    c = cells(ln)
    if state == "out":
        if c != HEADER:
            fail(i, f"表の見出しが「| 決まり | 守らせるもの |」でない: {ln}")
        state = "sep"
        continue
    if state == "sep":
        if len(c) != 2 or not all(re.fullmatch(r":?-+:?", x) for x in c):
            fail(i, f"見出しの次が 2 列の区切りの行でない: {ln}")
        state = "body"
        continue
    if len(c) != 2:
        fail(i, f"行が 2 列（決まり｜守らせるもの）でない（{len(c)} 列）: {ln}")
        continue
    rule, guard = c
    rows += 1
    if not rule:
        fail(i, "決まりが空")
    if not guard:
        fail(i, f"守らせるものが空（名前か「無し。」と理由を書く）: {rule}")
        continue
    if guard.startswith("無し"):
        nones += 1
        if not re.fullmatch(r"無し。\s*\S.*", guard):
            fail(i, f"守らせるものが「無し」なのに、「無し。」に続く理由が無い: {guard}")
        continue
    if "`" not in guard:
        fail(i, f"守らせるものに `…` で名指した検査・command・hook・設定が無い: {guard}")
    for w in named(guard):
        p = w.rstrip("/")
        if p.startswith("/") or ".." in p.split("/") or not os.path.exists(os.path.join(root, p)):
            fail(i, f"守らせるものが名指したファイルが repo に無い: {w}")

if rows == 0:
    fail(1, "「| 決まり | 守らせるもの |」の表の行が 1 つも無い")
for m in fails:
    print(f"FAIL {m}")
if not fails:
    print(f"ok   AGENTS.md の決まり {rows} 行が全部「決まり｜守らせるもの」の対で、名指したファイルが在る（守らせるものが無い行は {nones}）")
sys.exit(1 if fails else 0)
PY
}

case "${1:-}" in
  --root) check "$2"; exit ;;
  '') ;;
  *) sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2 ;;
esac

fail=0
check . || fail=1

# 自己確認（受け入れ条件 10）: 写しの根は AGENTS.md だけを写し、ほかは repo へのリンクにする（名指したファイルを本物で見る）
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
setup() {
  rm -rf "$tmp/r"; mkdir -p "$tmp/r"
  for p in * .[!.]*; do [ -e "$p" ] && [ "$p" != AGENTS.md ] && [ "$p" != .git ] && ln -s "$PWD/$p" "$tmp/r/$p"; done
  cp AGENTS.md "$tmp/r/AGENTS.md"
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
row='| `tests/run` を同時に2本走らせない（mise の置き場と CPU を取り合う）。 |'
grep -qF "$row" AGENTS.md || { echo "FAIL 自己確認の前提: AGENTS.md に「tests/run を同時に2本」の行が無い"; exit 1; }
grep -qF '| `tests/public.sh`。' AGENTS.md || { echo "FAIL 自己確認の前提: AGENTS.md に tests/public.sh を名指す行が無い"; exit 1; }

setup; expect ok '壊していない写しは通る'
setup; printf '%s\n' '- 新しい決まりを箇条書きで足す。' >>"$tmp/r/AGENTS.md"
expect fail '表の外に箇条書きを足した写しで落ちる' '表の外に書いた行'
setup; sed -i "s/^\(| \`tests\/run\` を同時に2本.*。\) | 無し。[^|]* |$/\1 |  |/" "$tmp/r/AGENTS.md"
expect fail '守らせるものを空にした写しで落ちる' '守らせるものが空'
setup; sed -i '0,/| `tests\/public.sh`。/s//| `tests\/no-such-check.sh`。/' "$tmp/r/AGENTS.md"
expect fail '在りえない検査を名指した写しで落ちる' '名指したファイルが repo に無い: tests/no-such-check.sh'
setup; sed -i "s/^\(| \`tests\/run\` を同時に2本.*。\) | 無し。[^|]* |$/\1 | 無し |/" "$tmp/r/AGENTS.md"
expect fail '「無し」の理由を消した写しで落ちる' '「無し。」に続く理由が無い'

exit "$fail"
