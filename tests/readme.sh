#!/usr/bin/env bash
# tests/readme.sh [--root <dir>]
#
# 英語の README.md と日本語の README.ja.md の構成がずれていないこと（ADR-0046 決めたこと 10、受け入れ条件 2）。
#   - 2 つのファイルがあり、冒頭 10 行以内に互いへのリンクがある。
#   - 見出しのレベルの並びが同じ（コードブロックの中は見出しに数えない）。
#   - 相対リンク（Markdown の [..](..)・[..](<..>)・[..](.. "title")・参照の定義 [..]: .. と、HTML の src・srcset・href）の先の
#     集合が同じで（互いへのリンクは除く）、先が全部 repo に在る。/ で始まる先は repo の根から読む。インラインのコードの中は見ない。
#     ../../pulls など GitHub の repo のページを指すリンクは、集合の比べには入れ、在るかは見ない。ほかの根の外へ出るリンクは落ちる。
#     ページの中の見出しへのリンク（#…）は、見出しが言語で違うので集合から外し、数が同じことだけ見る。
#   - コードブロック（Mermaid を除く）は、コメント（# から行末）と置き換えの印（<…>）を除いた command の行が同じ。
#   - Mermaid のブロックは数が同じで、引用符の中（ラベル）を除いた行の並びが同じ。最初のブロックが先頭から 40 行以内に始まる。
#   - 表示名の行（**Andon — a one-person software factory.**）と注記（> [!NOTE]）が、どちらにもある。
#   - ADR の一覧の行（- [NNNN で始まる行）が無い。
# 文の意味のずれは見ない（同じ PR で両方を直す決まりとレビューに任せる）。
# 引数なしは repo を見たあと、壊した写しでそれぞれ落ち、壊していない写しで通ることを確かめる。
#   --root <dir>  その木を見る（自己確認の写しに使う）。
set -euo pipefail
cd "$(dirname "$0")/.."

check() { # check <root>
  python3 - "$1" <<'PY'
import os, re, sys
from urllib.parse import unquote

root = sys.argv[1]
# README は repo の根にあるので、GitHub は ../../<ページ> を repo のページ（https://github.com/<owner>/<repo>/<ページ>）と読む
GITHUB = re.compile(r"^\.\./\.\./(?:pulls|issues|releases|tags|actions|discussions|wiki)(?:/[^.]*)?$")
EN, JA = "README.md", "README.ja.md"
NAME = "**Andon — a one-person software factory.**"
errs = []

def read(name):
    p = os.path.join(root, name)
    if not os.path.isfile(p):
        errs.append(f"{name} が無い")
        return None
    with open(p, encoding="utf-8") as f:
        return f.read().split("\n")

def blocks(lines):
    """コードブロックを (言語, 始まりの行番号, 中の行) で返し、ブロックの外の行を (行番号, 行) で返す"""
    out, prose, cur, fence = [], [], None, ""
    for n, line in enumerate(lines, 1):
        m = re.match(r"^\s{0,3}(`{3,}|~{3,})\s*([^\s`]*)", line)
        if cur is None and m:
            cur, fence = (m.group(2).lower(), n, []), m.group(1)
        elif cur is not None and re.match(r"^\s{0,3}" + re.escape(fence[0]) + "{" + str(len(fence)) + r",}\s*$", line):
            out.append(cur); cur = None
        elif cur is not None:
            cur[2].append(line)
        else:
            prose.append((n, line))
    if cur is not None:
        errs.append(f"閉じていないコードブロック（{cur[1]} 行目）")
    return out, prose

# [..](先) と [..](<先>)・[..](先 "title")、行頭の参照の定義 [..]: 先、HTML の src・srcset・href（" か '）
LINK = re.compile(r"\]\(\s*(?:<([^<>]+)>|([^()\s]+))(?:\s+(?:\"[^\"]*\"|'[^']*'))?\s*\)"
                  r"|^\s{0,3}\[[^\]]+\]:\s*(?:<([^<>]+)>|(\S+))"
                  r"|\b(src|srcset|href)=(?:\"([^\"]+)\"|'([^']+)')")

def links(prose):
    rel, anchors = [], 0
    for _, line in prose:
        line = re.sub(r"`[^`]*`", "", line)  # インラインのコードの中はリンクでない
        for m in LINK.finditer(line):
            if m.group(5) == "srcset":  # srcset は「先 2x, 先 1x」の候補を全部
                ts = [c.split()[0] for c in (m.group(6) or m.group(7)).split(",") if c.strip()]
            else:
                ts = [next(g for g in (m.groups()[:4] + m.groups()[5:]) if g)]
            for t in ts:
                if re.match(r"^[a-z][a-z0-9+.-]*:", t):  # http:・mailto: など
                    continue
                if t.startswith("#"):
                    anchors += 1
                    continue
                rel.append(t)
    return rel, anchors

docs = {}
for name in (EN, JA):
    lines = read(name)
    if lines is None:
        continue
    bl, prose = blocks(lines)
    docs[name] = dict(lines=lines, blocks=bl, prose=prose)

if len(docs) == 2:
    for name, other in ((EN, JA), (JA, EN)):
        d = docs[name]
        head = "\n".join(d["lines"][:10])
        if f"]({other})" not in head:
            errs.append(f"{name} の冒頭 10 行に {other} へのリンクが無い")
        if not any(line.strip().startswith(NAME) for _, line in d["prose"]):
            errs.append(f"{name} に表示名の行（{NAME}）が無い")
        if not any(re.match(r"^\s*>\s*\[!NOTE\]\s*$", line) for _, line in d["prose"]):
            errs.append(f"{name} に注記（> [!NOTE]）が無い")
        for n, line in d["prose"]:
            if re.match(r"^\s*(?:[-*+]|[0-9]+[.)])\s+\[[0-9]{4} ", line):
                errs.append(f"{name}:{n}: ADR の一覧の行がある（一覧は docs/adr/README.md）")
        mer = [b for b in d["blocks"] if b[0] == "mermaid"]
        if not mer:
            errs.append(f"{name} に Mermaid の図が無い")
        elif mer[0][1] > 40:
            errs.append(f"{name} の最初の図が {mer[0][1]} 行目に始まる（40 行以内に置く）")
        rel, _ = links(d["prose"])
        for t in rel:
            path = unquote(re.split(r"[?#]", t)[0])
            if path in (EN, JA) or not path:
                continue
            if GITHUB.match(path):  # 根の上の GitHub の repo のページ（PR の一覧など）
                continue
            norm = os.path.normpath(path.lstrip("/"))  # / で始まるパスは GitHub では repo の根から
            if norm == ".." or norm.startswith("../"):
                errs.append(f"{name} のリンクが repo の外を指す: {t}")
            elif not os.path.exists(os.path.join(root, norm)):
                errs.append(f"{name} のリンクの先が repo に無い: {t}")

    en, ja = docs[EN], docs[JA]
    def heads(d):
        return [len(m.group(1)) for _, line in d["prose"] if (m := re.match(r"^ {0,3}(#{1,6})(?:\s|$)", line))]
    if heads(en) != heads(ja):
        errs.append(f"見出しのレベルの並びが違う: {EN} {heads(en)} / {JA} {heads(ja)}")
    def relset(d):
        rel, anchors = links(d["prose"])
        return {t for t in rel if re.split(r"[?#]", t)[0] not in (EN, JA)}, anchors
    (er, ea), (jr, ja_a) = relset(en), relset(ja)
    if er != jr:
        errs.append(f"相対リンクの先が違う: {EN} だけ {sorted(er - jr)} / {JA} だけ {sorted(jr - er)}")
    if ea != ja_a:
        errs.append(f"ページの中の見出しへのリンクの数が違う: {EN} {ea} / {JA} {ja_a}")
    def code(d):
        out = []
        for lang, _, body in d["blocks"]:
            if lang == "mermaid":
                continue
            out.append([lang] + [re.sub(r"<[^<>]*>", "", re.sub(r"\s*#.*$", "", l)).rstrip() for l in body])
        return out
    if code(en) != code(ja):
        errs.append(f"コードブロックの command が違う: {EN} {code(en)} / {JA} {code(ja)}")
    def mermaid(d):
        return [[re.sub(r'"[^"]*"', '""', l).rstrip() for l in body] for lang, _, body in d["blocks"] if lang == "mermaid"]
    me, mj = mermaid(en), mermaid(ja)
    if len(me) != len(mj):
        errs.append(f"Mermaid の図の数が違う: {EN} {len(me)} / {JA} {len(mj)}")
    else:
        for i, (a, b) in enumerate(zip(me, mj), 1):
            if a != b:
                diff = [f"{x!r} / {y!r}" for x, y in zip(a + [""] * len(b), b + [""] * len(a)) if x != y][:3]
                errs.append(f"{i} 枚目の図のノードと矢印が違う（ラベルを除いて）: {'; '.join(diff)}")

for e in errs:
    print(f"FAIL readme: {e}")
if errs:
    sys.exit(1)
print(f"ok   readme: {EN} と {JA} の見出し・相対リンク・command・図・表示名・注記が揃っている")
PY
}

if [ "${1:-}" = --root ]; then
  [ -n "${2:-}" ] || { echo "使い方: tests/readme.sh [--root <dir>]" >&2; exit 2; }
  check "$2"
  exit
fi

fail=0
check . || fail=1

# 自己確認（ADR-0046 受け入れ条件 2）: 写しの根は 2 つの README だけを写し、ほかは repo へのリンクにする（リンクの先を本物で見る）
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
setup() {
  rm -rf "$tmp/r"; mkdir -p "$tmp/r"
  for p in * .[!.]*; do
    case "$p" in README.md | README.ja.md | .git) ;; *) [ -e "$p" ] && ln -s "$PWD/$p" "$tmp/r/$p" ;; esac
  done
  cp README.md README.ja.md "$tmp/r/"
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
# 写しを壊す行が README に在ること（無ければ、壊したつもりの写しが壊れていない）
need() { # need <ファイル> <固定の行>
  grep -qxF -- "$2" "$1" || { echo "FAIL 自己確認の前提: $1 に「$2」の行が無い"; exit 1; }
}
need README.ja.md '## 開発と履歴'
need README.ja.md 'go test ./...'
need README.ja.md '  lamp --> owner'
need README.ja.md '> [!NOTE]'
need README.md 'English | [日本語](README.ja.md)'
grep -qF '](docs/vision.md)' README.md || { echo "FAIL 自己確認の前提: README.md に docs/vision.md へのリンクが無い"; exit 1; }
grep -qF '](docs/vision.md)' README.ja.md || { echo "FAIL 自己確認の前提: README.ja.md に docs/vision.md へのリンクが無い"; exit 1; }
grep -q '^\*\*Andon — a one-person software factory\.\*\*' README.md || { echo "FAIL 自己確認の前提: README.md の表示名の行が無い"; exit 1; }
grep -qF '](../../pulls?' README.md || { echo "FAIL 自己確認の前提: README.md に根の上へ出るリンクが無い"; exit 1; }
[ ! -e docs/no-such-doc.md ] || { echo "FAIL 自己確認の前提: docs/no-such-doc.md が在る"; exit 1; }

setup; expect ok '壊していない写しは通る（根の上へ出るリンク ../../pulls も通る）'
setup; sed -i '/^## 開発と履歴$/d' "$tmp/r/README.ja.md"; expect fail '片方の見出しを 1 つ消した写しで落ちる' '見出しのレベルの並びが違う'
setup; sed -i 's|](docs/vision.md)|](docs/no-such-doc.md)|' "$tmp/r/README.md" "$tmp/r/README.ja.md"
expect fail '相対リンクを在りえないパスにした写しで落ちる' 'リンクの先が repo に無い: docs/no-such-doc.md'
setup; sed -i 's|](docs/vision.md)|](docs/design.md)|' "$tmp/r/README.ja.md"; expect fail '片方の相対リンクの先を替えた写しで落ちる' '相対リンクの先が違う'
setup; sed -i 's|^go test \./\.\.\.$|go test ./cmd/...|' "$tmp/r/README.ja.md"; expect fail '片方のコードブロックの command を変えた写しで落ちる' 'コードブロックの command が違う'
setup; sed -i 's|^\(tests/run --changed\) .*|\1     # 別の注釈|' "$tmp/r/README.ja.md"; expect ok '片方のコードブロックの注釈だけを変えた写しは通る'
setup; sed -i '/^  lamp --> owner$/d' "$tmp/r/README.ja.md"; expect fail '片方の図の矢印を 1 本消した写しで落ちる' '1 枚目の図のノードと矢印が違う'
setup; sed -i 's/^  owner(\["オーナー"\])$/  owner(["持ち手"])/' "$tmp/r/README.ja.md"; expect ok '片方の図のラベルだけを変えた写しは通る'
setup; sed -i '/^English | \[日本語\](README.ja.md)$/d' "$tmp/r/README.md"; expect fail '互いへのリンクを消した写しで落ちる' 'README.md の冒頭 10 行に README.ja.md へのリンクが無い'
setup; sed -i '/^> \[!NOTE\]$/d' "$tmp/r/README.ja.md"; expect fail '片方の注記を消した写しで落ちる' 'README.ja.md に注記'
setup; sed -i '/^\*\*Andon — a one-person software factory\.\*\*/d' "$tmp/r/README.md"; expect fail '片方の表示名の行を消した写しで落ちる' 'README.md に表示名の行'
setup; printf -- '- [0001 skill の配り方](docs/adr/0001-skills-distribution.md): 決定\n' | tee -a "$tmp/r/README.md" >>"$tmp/r/README.ja.md"
expect fail 'ADR の一覧の行を足した写しで落ちる' 'ADR の一覧の行がある'
setup; for f in README.md README.ja.md; do sed -i '0,/^## /s//\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n## /' "$tmp/r/$f"; done
expect fail '図を 41 行目より後ろへ動かした写しで落ちる' '行目に始まる（40 行以内に置く）'
# リンクと一覧の行の書き方の変種（両方の README に足して、集合の比べでなく在るかの検査で落ちることを見る）
both() { printf '%s\n' "$1" | tee -a "$tmp/r/README.md" >>"$tmp/r/README.ja.md"; }
setup; both '[x](docs/no-such-doc.md "title")'; expect fail 'title つきのリンクの在りえない先で落ちる' 'リンクの先が repo に無い: docs/no-such-doc.md'
setup; both '[x]: docs/no-such-doc.md'; expect fail '参照の定義の在りえない先で落ちる' 'リンクの先が repo に無い: docs/no-such-doc.md'
setup; both "<img src='docs/no-such-doc.svg'>"; expect fail '単引用符の src の在りえない先で落ちる' 'リンクの先が repo に無い: docs/no-such-doc.svg'
setup; both '<img srcset="docs/assets/logo/andon-wordmark.svg 1x, docs/no-such-doc.svg 2x">'
expect fail 'srcset の 2 つ目の候補の在りえない先で落ちる' 'リンクの先が repo に無い: docs/no-such-doc.svg'
setup; printf '%s\n' '<img srcset="docs/assets/logo/andon-wordmark.svg 1x, docs/assets/logo/andon-mark.svg 2x">' >>"$tmp/r/README.md"
printf '%s\n' '<img srcset="docs/assets/logo/andon-wordmark.svg 1x">' >>"$tmp/r/README.ja.md"
expect fail '片方の srcset にだけ候補がある写しで落ちる' '相対リンクの先が違う'
setup; both '[x](/tmp)'; expect fail '/ で始まるリンクは repo の根から読み、根に無い先で落ちる' 'リンクの先が repo に無い: /tmp'
setup; both '[x](../no-such.md)'; expect fail 'GitHub のページでない根の外のリンクで落ちる' 'リンクが repo の外を指す: ../no-such.md'
setup; both '[x](/docs/design.md "title") `[y](docs/no-such-doc.md)`'; expect ok '/ で始まる在る先と、インラインのコードの中は通る'
setup; both '* [0001 skill の配り方](docs/adr/0001-skills-distribution.md): 決定'; expect fail '* で始まる ADR の一覧の行で落ちる' 'ADR の一覧の行がある'
setup; sed -i 's/^## 開発と履歴$/  ## 開発と履歴/' "$tmp/r/README.ja.md"; expect ok '行頭に 3 字までの空白を置いた見出しも見出しに数える'
setup; sed -i 's/^## 開発と履歴$/  ### 開発と履歴/' "$tmp/r/README.ja.md"; expect fail '行頭に空白を置いた見出しのレベルが違う写しで落ちる' '見出しのレベルの並びが違う'
setup; sed -i '0,/^```bash$/s//```text/' "$tmp/r/README.ja.md"; expect fail '片方のコードブロックの言語を替えた写しで落ちる' 'コードブロックの command が違う'
setup; rm "$tmp/r/README.ja.md"; expect fail 'README.ja.md の無い写しで落ちる' 'README.ja.md が無い'
setup; sed -i 's|(#the-cell)|(#the-cell) ([again](#the-cell))|' "$tmp/r/README.md"; expect fail 'ページの中の見出しへのリンクの数が違う写しで落ちる' '見出しへのリンクの数が違う'

exit "$fail"
