#!/usr/bin/env bash
# tests/shell-sigpipe.sh [--list] [<ファイル> …]
#
# set -o pipefail の下で、pipe の後ろに出力の途中で抜ける読み手（head・grep -q・grep -m・grep -l）を置くと、読み手が先に終わった
# あとに前のコマンドが書くと SIGPIPE で落ち、pipe 全体が失敗になる。前のコマンドの出力が伸びたときだけ表に出るので、検査が
# 自分の差分と無関係な所で誤って落ちる。tests/ の下のシェルで、pipefail を set するファイルのそのような行を探し、ファイルと行を
# 出して落ちる。直し方は、前のコマンドの出力を $( … ) で受けてから読む（grep -q x <<<"$out"）か、終わりまで読む読み手
# （sed -n 1p・grep -c）にする。
# 前からある行は控え tests/shell-sigpipe.allow（「<ファイル><TAB><行>」）に載せてあり、拾わない。控えは減らすだけで、新しい行を
# 足さない。控えの行がもう無ければ（直した・書き換えた）、控えから消すよう言って落ちる。行は注釈を除き、空白を1つに詰めて比べる。
# 害が無いと分かっている新しい行は、その行の注釈に「shell-sigpipe: ok <理由>」と書く。
# 見るのは、set の行に pipefail のあるファイルの、here-doc の外の、一重引用符の外のコード（"$( … )" の中は見る）。
# 引数なしは、自分の読み手を見本で確かめてから、tests/ の下のシェルを全部見る（git が追跡するか、まだ add していない新しい
# ファイルのうち、*.sh と先頭行が bash か sh の #! のもの）。引数を渡すと、そのファイルだけを見る（控えは当てる。控えの古さは
# 見ない）。--list は、見たファイル（引数なしは tests/ の下のシェルの全部）の拾った行を控えの形で出す（控えを当てない）。
set -euo pipefail
cd "$(dirname "$0")/.."
allow=tests/shell-sigpipe.allow
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

scan() { # scan <ファイル> …: 拾った行を「<ファイル>:<行>:<TAB><詰めた行>」で出す。読めないファイルがあれば終了コード 2
  python3 - "$@" <<'PY'
import re, sys

PIPEFAIL = re.compile(r"^\s*(\(\s*)?set\s+[^#\n]*?(-o\s+pipefail|-[a-zA-Z]*o\s+pipefail)\b")
# here-doc の始まり。区切りは引用符の中身か、空白・演算子までの語（<<EOF-X は EOF-X）
HEREDOC = re.compile(r"(?<!<)<<(?!<)(-?)[ \t]*(?:'([^']*)'|\"([^\"]*)\"|\\?([^\s;&|<>()'\"]+))")
PIPE = re.compile(r"(?<![|])\|&?(?![|])")
PREFIX = re.compile(r"\s*(?:[A-Za-z_][A-Za-z0-9_]*=\S*\s+|command\s+|env\s+|timeout\s+\S+\s+)*")
READER = re.compile(r"(?:\S*/)?(head|grep|egrep|fgrep)(?=\s|$|;)")
EARLY_SHORT = set("qmlL")
ARG_SHORT = set("efmABCD")
EARLY_LONG = ("--quiet", "--silent", "--max-count", "--files-with-matches", "--files-without-match")


def strip_line(line, stack):
    """1行を読み、(コード, here-doc を探す文字, stack) を返す。
    コードは一重引用符の中と注釈を落とした行（"$( … )" の中は残す）。here-doc を探す文字は、コマンドの位置（引用符の外か
    $( … ) の中）の文字だけで、二重引用符の中と算術（$(( … ))・(( … ))）の中を除く。一重引用符の中身は残す（<<'EOF' の区切り）。
    stack は行をまたぐ入れ子: "'"・"'k"（here-doc の区切り。中身を残す）・"$'"（一重引用符）・'"'（二重引用符）・"("（$( … ) と、その中の ( … )）・"A"（算術）"""
    out, hd = [], []
    i, n = 0, len(line)
    while i < n:
        c = line[i]
        top = stack[-1] if stack else None
        if top in ("'", "'k"):
            if c == "'":
                stack.pop()
                out.append("''")
            if top == "'k" or c == "'":  # 中身を残すのは here-doc の区切り（<<'EOF'）だけ
                hd.append(c)
            i += 1
            continue
        if top == "$'":
            if c == "\\":
                i += 2
                continue
            if c == "'":
                stack.pop()
                out.append("''")
            i += 1
            continue
        if top in ('"', "A"):
            if c == "\\":
                out.append(line[i:i + 2])
                i += 2
                continue
            if top == '"' and c == '"':
                stack.pop()
            elif line.startswith("$((", i):
                stack.append("A")
                out.append("$((")
                i += 3
                continue
            elif line.startswith("$(", i):
                stack.append("(")
                out.append("$(")
                i += 2
                continue
            elif top == "A" and line.startswith("))", i):
                stack.pop()
                out.append("))")
                i += 2
                continue
            elif top == "A" and c == "(":
                stack.append("(")
            out.append(c)
            i += 1
            continue
        # コマンドの位置（引用符の外か $( … ) の中）
        if c == "\\":
            out.append(line[i:i + 2])
            hd.append(line[i:i + 2])
            i += 2
            continue
        if c == "'":
            stack.append("'k" if re.search(r"<<-?[ \t]*$", "".join(hd)) else "'")
            hd.append(c)
            i += 1
            continue
        if c == "$" and line[i + 1:i + 2] == "'":
            stack.append("$'")
            i += 2
            continue
        if c == '"':
            stack.append('"')
            out.append(c)
            i += 1
            continue
        if line.startswith("$((", i) or line.startswith("((", i):
            k = 3 if c == "$" else 2
            stack.append("A")
            out.append(line[i:i + k])
            i += k
            continue
        if line.startswith("$(", i):
            stack.append("(")
            out.append("$(")
            hd.append("$(")
            i += 2
            continue
        if c == "(" and top == "(":
            stack.append("(")
        elif c == ")" and top == "(":
            stack.pop()
        elif c == "#" and (i == 0 or line[i - 1] in " \t;|&("):
            break
        out.append(c)
        hd.append(c)
        i += 1
    return "".join(out), "".join(hd), stack


def early_reader(code):
    for m in PIPE.finditer(code):
        rest = code[m.end():]
        p = PREFIX.match(rest)
        rest = rest[p.end():]
        r = READER.match(rest)
        if not r:
            continue
        if r.group(1) == "head":
            return True
        skip = False
        for tok in rest[r.end():].split():
            if skip:  # 前の短い option（-e・-f など）の値
                skip = False
                continue
            if tok in (";", "|", "||", "&&", ")", "&") or tok.startswith((";", "|", "&", ")")):
                break
            if tok == "--":
                break
            if tok.startswith("--"):
                if tok.startswith(EARLY_LONG):
                    return True
                continue
            if tok.startswith("-") and len(tok) > 1:
                for k, ch in enumerate(tok[1:], 1):
                    if ch in EARLY_SHORT:
                        return True
                    if ch in ARG_SHORT:
                        skip = k == len(tok) - 1  # 値が次の語
                        break
                continue
            # パターンかファイル。GNU grep はこの後ろの option も読む（grep x -q）ので、-- か演算子まで見る
    return False


bad = False
for path in sys.argv[1:]:
    try:
        text = open(path, encoding="utf-8").read()
    except (OSError, UnicodeDecodeError) as e:
        print(f"{path}: 読めない: {e}", file=sys.stderr)
        bad = True
        continue
    lines = text.split("\n")
    if not any(PIPEFAIL.match(l) and "+o pipefail" not in l for l in lines):
        continue
    stack = []
    heredocs = []  # (区切り, タブを除くか)
    logical, raw = [], []
    for no, line in enumerate(lines, 1):
        if heredocs:
            word, tabs = heredocs[0]
            if (line.lstrip("\t") if tabs else line) == word:
                heredocs.pop(0)
            continue
        raw.append(line)
        code, hd, stack = strip_line(line, stack)
        logical.append(code)
        for m in HEREDOC.finditer(hd):
            heredocs.append((next(g for g in m.group(2, 3, 4) if g is not None), m.group(1) == "-"))
        if stack:
            continue
        stripped = code.rstrip()
        if stripped.endswith("\\") and not stripped.endswith("\\\\"):
            logical[-1] = stripped[:-1]
            continue
        if stripped.endswith("|") and not stripped.endswith("||"):
            continue
        joined = " ".join(" ".join(logical).split())
        if early_reader(joined) and not any("shell-sigpipe: ok" in r for r in raw):
            print(f"{path}:{no}:\t{joined}")
        logical, raw = [], []
    if stack or heredocs:
        what = "閉じていない here-doc（" + heredocs[0][0] + "）" if heredocs else "閉じていない引用符か $( … )"
        print(f"{path}: 読めない: {what}", file=sys.stderr)
        bad = True
sys.exit(2 if bad else 0)
PY
}

die() { echo "FAIL $*" >&2; exit 1; }

# apply <拾った行のファイル>: 控えに無い行を出して 1、無ければ 0。控えの行は数の分だけ打ち消す
apply() {
  python3 - "$1" "$allow" <<'PY'
import sys
from collections import Counter
allow = Counter()
for l in open(sys.argv[2], encoding="utf-8"):
    l = l.rstrip("\n")
    if l:
        allow[l] += 1
bad = False
for l in open(sys.argv[1], encoding="utf-8"):
    l = l.rstrip("\n")
    head, _, code = l.partition("\t")
    key = head.split(":")[0] + "\t" + code
    if allow[key] > 0:
        allow[key] -= 1
        continue
    print(l)
    bad = True
sys.exit(1 if bad else 0)
PY
}

# check <ファイル> …: 控えに無い拾った行を出して落ちる
check() {
  local rc=0
  scan "$@" >"$tmp/found" || rc=$?
  [ "$rc" = 0 ] || die "読めないファイルがある（終了コード $rc）"
  if ! apply "$tmp/found" >"$tmp/new"; then
    cat "$tmp/new"
    die "pipefail の下で、pipe の後ろに途中で抜ける読み手（head・grep -q・grep -m・grep -l）がある。前のコマンドの出力が伸びると SIGPIPE で落ちる。出力を \$( … ) で受けてから読む（grep -q x <<<\"\$out\"）か、終わりまで読む読み手（sed -n 1p・grep -c）にする"
  fi
}

# tests/ の下で git が追跡するか、まだ add していない新しいシェルを files に入れる
list_files() {
  files=()
  local f first
  while IFS= read -r f; do
    case "$f" in
      *.sh) files+=("$f") ;;
      *) if [ -f "$f" ]; then
        first=$(sed -n 1p "$f")
        if [[ $first =~ ^#!.*[/\ ](bash|sh)([[:space:]]|$) ]]; then files+=("$f"); fi
      fi ;;
    esac
  done < <(git ls-files --cached --others --exclude-standard tests)
  [ "${#files[@]}" -ge 50 ] || die "見るファイルが少なすぎる（${#files[@]} 本）"
}

if [ "${1:-}" = --list ]; then
  shift
  if [ $# -eq 0 ]; then
    list_files
    set -- "${files[@]}"
  fi
  scan "$@" | sed 's/^\([^:]*\):[0-9]*:\t/\1\t/'
  exit 0
fi
if [ $# -gt 0 ]; then
  check "$@"
  echo "ok shell-sigpipe: $# ファイル"
  exit 0
fi

# 見本: 拾う形（行に「# 拾う」と書いた行）と拾わない形。here-doc の中なので、この検査自身の見る対象にはならない
cat >"$tmp/sample.sh" <<'SAMPLE'
#!/usr/bin/env bash
set -euo pipefail
[ -d "$app/.git" ] && git -C "$app" worktree list --porcelain | head -1 | grep -qx "worktree $app" || die '本体の checkout が消えた' # 拾う
sed -n '/^## 動かし方$/,/^## /p' "$f" | grep -q '^前提:$' || die "$f の動かし方が「前提:」で始まらない" # 拾う
[ "$(git worktree list --porcelain | head -1)" = "worktree $app" ] || die x # 拾う
x=$(seq 100000 | head -n 3) # 拾う
seq 10 | grep -qF 3 || die x # 拾う
seq 10 | grep -vqE 3 && die x # 拾う
seq 10 | grep -m1 3 # 拾う
seq 10 | grep -m 1 3 # 拾う
seq 10 | grep --quiet 3 # 拾う
seq 10 | grep --max-count=1 3 # 拾う
seq 10 | grep -l 3 # 拾う
seq 10 | grep 1 -q # 拾う（GNU grep はパターンの後ろの option も読む）
seq 10 | grep 1 -m 1 f # 拾う
seq 10 | LC_ALL=C grep -q 3 # 拾う
seq 10 | /usr/bin/grep -q 3 # 拾う
seq 10 |& head -2 # 拾う
if seq 10 | grep -q 3; then :; fi # 拾う
seq 10 |
  grep -q 3 # 拾う（並びの終わりの行で出す）
seq 10 \
  | head -1 # 拾う
echo "$(seq 10 | head -1)" # 拾う
seq 10 | grep -c 3
seq 10 | grep -e -q 3
seq 10 | grep -f pats
seq 10 | sed -n 1p
out=$(seq 10); grep -q 3 <<<"$out"
seq 10 || head -1 f
seq 10 | grep -q 3 # shell-sigpipe: ok 出力は 1 回の書き込み
echo 'seq 10 | head -1'
jq -r '.a | head' f
# seq 10 | head -1
echo "x" # seq 10 | head -1
case $x in a|head) : ;; esac
cat <<EOF
seq 10 | head -1
EOF
cat <<-'EOF' >"$tmp/x"
	seq 10 | grep -q 3
	EOF
python3 -c '
import sys; print(sys.argv) | head
'
a="$(tr -d '"' <f)"
git log | head -1 # 拾う（"$( … )" の中の一重引用符は引用符）
b="$(tr -d '"' <g)"
n=$(( 1 << width ))
git log | head -2 # 拾う（算術の << は here-doc でない）
echo "usage <<EOF>>"
git log | head -3 # 拾う（二重引用符の中の << は here-doc でない）
cat <<EOF-X
seq 10 | head -1
EOF-X
git log | head -4 # 拾う（区切りは空白まで）
out=$(cat <<EOF
seq 10 | head -1
EOF
)
x=$(
  seq 10 | head -1
) # 拾う（複数行の $( … ) は終わりの行で出す）
SAMPLE
grep -n '# 拾う' "$tmp/sample.sh" | cut -d: -f1 | sed "s|^|$tmp/sample.sh:|" >"$tmp/want"
scan "$tmp/sample.sh" | cut -d: -f1-2 >"$tmp/got" || die "見本を読めない"
[ "$(wc -l <"$tmp/want")" -ge 15 ] || die "見本の拾う行が少なすぎる（$(wc -l <"$tmp/want") 行）"
diff "$tmp/want" "$tmp/got" >&2 || die "見本で拾う行が違う（< 拾うはずの行・> 拾った行）"
# pipefail を set しないファイルは見ない。set の書き方の違いでも拾う
printf '#!/usr/bin/env bash\nset -eu\nseq 10 | head -1\n' >"$tmp/no-pf.sh"
[ -z "$(scan "$tmp/no-pf.sh")" ] || die "pipefail の無いファイルを見た"
for opts in '-o pipefail' '-euo pipefail' '-e -o pipefail' '-eo pipefail'; do
  printf '#!/usr/bin/env bash\nset %s\nseq 10 | head -1\n' "$opts" >"$tmp/opts.sh"
  [ "$(scan "$tmp/opts.sh" | cut -d: -f2)" = 3 ] || die "set $opts のファイルで拾わない"
done
# 読めない形は落ちる（黙って何も見ずに通らない）
printf '#!/usr/bin/env bash\nset -o pipefail\necho "x\n' >"$tmp/broken.sh"
if scan "$tmp/broken.sh" >/dev/null 2>"$tmp/err"; then die "閉じていない引用符のファイルを読めたことにした"; fi
grep -q '読めない' "$tmp/err" || die "読めないときの理由が出ない"
printf '#!/usr/bin/env bash\nset -o pipefail\ncat <<EOF\nx\n' >"$tmp/open-heredoc.sh"
if scan "$tmp/open-heredoc.sh" >/dev/null 2>"$tmp/err"; then die "閉じていない here-doc のファイルを読めたことにした"; fi
grep -q 'here-doc' "$tmp/err" || die "閉じていない here-doc の理由が出ない"
# 控え: 載っている行は拾わない。同じ行を1つ足すと拾う。行を書き換えると拾う
mkdir -p "$tmp/tests"
printf '#!/usr/bin/env bash\nset -o pipefail\nseq 10 | head -1\n' >"$tmp/tests/a.sh"
scan "$tmp/tests/a.sh" | sed 's/^\([^:]*\):[0-9]*:\t/\1\t/' >"$tmp/allow"
[ "$(wc -l <"$tmp/allow")" = 1 ] || die "見本の控えが 1 行でない"
( allow=$tmp/allow; scan "$tmp/tests/a.sh" >"$tmp/f1"; apply "$tmp/f1" >/dev/null ) || die "控えに載った行を拾った"
printf 'seq 10 | head -1\n' >>"$tmp/tests/a.sh"
if ( allow=$tmp/allow; scan "$tmp/tests/a.sh" >"$tmp/f2"; apply "$tmp/f2" >"$tmp/out" ); then die "控えに 1 つだけ載った行の 2 つ目を拾わない"; fi
grep -q 'a.sh:4:' "$tmp/out" || die "足した行の場所が出ない: $(cat "$tmp/out")"
printf '#!/usr/bin/env bash\nset -o pipefail\nseq 20 | head -1\n' >"$tmp/tests/a.sh"
if ( allow=$tmp/allow; scan "$tmp/tests/a.sh" >"$tmp/f3"; apply "$tmp/f3" >/dev/null ); then die "書き換えた行を拾わない"; fi
echo 'ok shell-sigpipe: 見本の拾う形と拾わない形、控えの当て方'

list_files
check "${files[@]}"
# 引数なしの --list は、ここで見たファイルと同じ一覧を出す
[ "$(bash "$0" --list | sort)" = "$(scan "${files[@]}" | sed 's/^\([^:]*\):[0-9]*:\t/\1\t/' | sort)" ] || die "引数なしの --list が、tests/ の下のシェルの拾った行を出さない"
# 控えの古さ: 控えの行が今のファイルに無ければ、控えから消す（減らすだけの控えにする）
scan "${files[@]}" | sed 's/^\([^:]*\):[0-9]*:\t/\1\t/' | sort >"$tmp/now"
sort "$allow" | sed '/^$/d' >"$tmp/allowed"
comm -13 "$tmp/now" "$tmp/allowed" >"$tmp/stale"
if [ -s "$tmp/stale" ]; then
  cat "$tmp/stale"
  die "$allow の上の行が、もう tests/ の下に無い。直したか書き換えた行なので、控えから消す"
fi
echo "ok shell-sigpipe: tests/ の下のシェル ${#files[@]} 本に、控えの外の pipe の後ろの途中で抜ける読み手が無い（控え $(wc -l <"$allow") 行）"
