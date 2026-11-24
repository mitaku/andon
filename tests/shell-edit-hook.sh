#!/usr/bin/env bash
# 検査のシェルを Bash で書き換える呼び出しを断る、この repo の Claude Code の PreToolUse のフック（.claude/hooks/test-shell-edit）。
# .claude/settings.json が Bash の PreToolUse でフックを指し、tests/ の下の .sh へ書く呼び出し（heredoc で包んだ python・node・perl の
# 書き込み、sed -i、perl -i、リダイレクト・tee）を deny して Edit ツールを使う理由の1行を返し、読むだけ・走らせるだけの呼び出しを通すこと。
# 断る形の初めの6つは、検査のシェルを書き換えて決まりに反した・壊した出来事の呼び出しを写したもの（パスと中身は縮めた）。
set -euo pipefail
cd "$(dirname "$0")/.."
fail=0
ok() { echo "ok   $1"; }
ng() { echo "FAIL $1"; fail=1; }
h=.claude/hooks/test-shell-edit
root=$PWD
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# フックの在りかと繋ぎ方。settings の command（Claude Code が sh で打つ）をそのまま使って当てる
hook_cmd=$(jq -r '[.hooks.PreToolUse[]? | select(.matcher == "Bash") | .hooks[]? | select(.type == "command") | .command | select(contains(".claude/hooks/test-shell-edit"))][0] // empty' .claude/settings.json 2>/dev/null || true)
if [ -n "$hook_cmd" ] && [ -x "$h" ] && python3 -c 'import ast, sys; ast.parse(open(sys.argv[1]).read())' "$h" 2>/dev/null; then
  ok ".claude/settings.json の PreToolUse（matcher Bash）が .claude/hooks/test-shell-edit を呼び、フックは実行できる"
else
  ng ".claude/settings.json に Bash の PreToolUse のフック（.claude/hooks/test-shell-edit）が無いか、フックが実行できない"
fi

# input <tool> <command>: PreToolUse の入力の JSON
input() { jq -n --arg t "$1" --arg c "$2" --arg d "$root" '{hook_event_name: "PreToolUse", tool_name: $t, cwd: $d, tool_input: {command: $c}}'; }
# pt <tool> <command>: settings の command でフックに入力を渡し、出力を返す（フックが無ければ空）
pt() {
  [ -n "$hook_cmd" ] || return 0
  input "$1" "$2" | CLAUDE_PROJECT_DIR=$root /bin/sh -c "$hook_cmd" 2>/dev/null || true
}
pt_deny() {
  pt Bash "$1" | jq -e '.hookSpecificOutput | .hookEventName == "PreToolUse" and .permissionDecision == "deny"
    and (.permissionDecisionReason | test("Edit ツール") and (contains("\n") | not))' >/dev/null 2>&1
}

# cases: 「%%」の行で区切った呼び出しを、1つずつ配列 cases に読む。呼び出しの中の heredoc の区切り（EOF など）とぶつからないよう、
# 包む heredoc の区切りは CASES にする。@ROOT@ は repo の根の絶対パスに置き換える
cases=()
read_cases() {
  cases=(); local c= line
  while IFS= read -r line; do
    line=${line//@ROOT@/$root}
    if [[ "$line" == "%%" ]]; then cases+=("$c"); c=; else c+="${c:+$'\n'}$line"; fi
  done
  [ -z "$c" ] || cases+=("$c")
}

# 断る: 出来事の形と、同じ種類の形
read_cases <<'CASES'
python3 - <<'EOF'
p='tests/image.sh'
s=open(p).read()
old='''dc exec -T dev bash -euc '
  cat >"$d/gh" <<EOF
#!/bin/sh
EOF
  rm -f "$d/gitconfig"
'
'''
open(p,'w').write(s.replace(old, old + '# (a0)\n'))
EOF
%%
cd @ROOT@; python3 - <<'PYEND'
import re
p='tests/package.sh'
s=open(p).read()
s,n=re.subn(r"<<<<<<< HEAD\n(.*?)=======\n(.*?)>>>>>>> [^\n]*\n",lambda m:m.group(1)+m.group(2),s,flags=re.S)
open(p,'w').write(s)
PYEND
grep -n '^#  19' tests/package.sh
%%
sed -i '2,3c\
# `andon digest`。一時の kanban で collect・build・send を通す。\
# 最後に本物の kanban の CLI で、ルーチンの例を写して lint・routine run・tick を通す。' tests/andon-digest.sh
head -7 tests/andon-digest.sh
%%
sed -i 's/kind: scope\\n  status: received/kind: scope\\n  asked: "%s"\\n  status: received/; s/answeredBy: terminal:t1\\n'"'"' "\$hour_ago"/answeredBy: terminal:t1\\n'"'"' "$hour_ago" "$hour_ago"/' tests/andon-digest.sh
grep -n "kind: scope" tests/andon-digest.sh; bash tests/andon-digest.sh
%%
s=$S
cat > "$s/docs.py" <<'EOF'
p=None
s=''
def op(x):
    global p,s
    if p: open(p,'w').write(s)
    p=x
    s=open(p).read() if p else ''
def r(a,b):
    global s
    assert s.count(a)==1,(p,a)
    s=s.replace(a,b)
op('tests/schema.sh')
r('ok "x"', 'ok "y"')
op(None)
EOF
python3 "$s/docs.py" && git status --short
%%
cd @ROOT@
python3 -I - <<'E'
p='tests/andon-create.sh'
s=open(p).read()
def r(a,b):
    global s
    assert s.count(a)==1,a
    s=s.replace(a,b)
r('ok "a"', 'ok "b"')
open(p,'w').write(s)
E
%%
sed -E -i 's/a/b/' ./tests/smoke.sh
%%
sed --in-place=.bak 's/a/b/' tests/words.sh
%%
sed -ni '1,5p' tests/lib/isolate.sh
%%
sed -i 's/a/b/' tests/*.sh
%%
sed -i "s/a/b/" "tests/smoke.sh"
%%
sed -i'' -e 's/a/b/' -- tests/smoke.sh
%%
sed -i 's/a/b/' @ROOT@/tests/ci.sh
%%
sed -i 's/a/b/' "${CLAUDE_PROJECT_DIR}/tests/ci.sh"
%%
xargs -n 1 sed -i 's/a/b/' tests/ci.sh
%%
env LC_ALL=C /usr/bin/sed -i 's/a/b/' tests/ci.sh
%%
n=$(sed -i 's/a/b/' tests/ci.sh && echo done)
%%
perl -pi -e 's/a/b/' tests/package/25-pgrep-full.sh
%%
perl -i.bak -pe 's/a/b/' tests/public.sh
%%
printf '%s\n' 'ok "x"' >> tests/smoke.sh
%%
cat > tests/lib/new.sh <<'EOF'
echo x
EOF
%%
bash tests/x.sh &> tests/x.sh
%%
echo x >| "$CLAUDE_PROJECT_DIR"/tests/x.sh
%%
printf 'x\n' | tee -a ./tests/smoke.sh >/dev/null
%%
python3 -c "p='tests/words.sh'; s=open(p).read(); open(p, 'w').write(s.replace('a', 'b'))"
%%
node -e "const fs=require('fs'); const p='tests/host.sh'; fs.writeFileSync(p, fs.readFileSync(p, 'utf8').replace('a', 'b'))"
%%
perl - <<'EOF'
open(my $f, '>', 'tests/smoke.sh') or die;
print $f "x\n";
EOF
%%
python3 - <<'EOF'
from pathlib import Path
p = Path('tests/words.sh')
p.write_text(p.read_text().replace('a', 'b'))
EOF
%%
python3 - tests/smoke.sh <<'EOF'
import sys
p = sys.argv[1]
open(p, 'w').write(open(p).read().replace('a', 'b'))
EOF
%%
python3 -c 'import sys; p = sys.argv[1]; open(p, "w").write("")' tests/smoke.sh
%%
node -e "require('fs').writeFileSync(process.argv[1], '')" tests/host.sh
%%
python3 - <<'EOF'
n = 'smoke'
p = f'tests/{n}.sh'
open(p, 'w').write('')
EOF
%%
python3 - <<'EOF'
import os
p = os.path.join('tests', 'smoke.sh')
open(p, 'w').write('')
EOF
%%
python3 - <<'EOF'
from pathlib import Path
(Path('tests') / 'smoke.sh').write_text('')
EOF
%%
python3 - <<'EOF'
import subprocess
subprocess.run(['sed', '-i', 's/a/b/', 'tests/smoke.sh'])
EOF
%%
bash <<'EOF'
sed -i 's/a/b/' tests/smoke.sh
EOF
%%
bash -c "sed -i 's/a/b/' tests/smoke.sh"
%%
sh -c 'echo x >> tests/smoke.sh'
%%
mise exec -- sed -i 's/a/b/' tests/smoke.sh
%%
sudo -u andon sed -i 's/a/b/' tests/smoke.sh
%%
env -u X sed -i 's/a/b/' tests/smoke.sh
%%
cd tests && sed -i 's/a/b/' smoke.sh
%%
cd @ROOT@/tests; perl -pi -e 's/a/b/' lib/isolate.sh
%%
echo "$(sed -i 's/a/b/' tests/smoke.sh)"
%%
x="$(printf x > tests/smoke.sh)"
%%
echo "`sed -i 's/a/b/' tests/smoke.sh`"
%%
node -e 'require("fs").writeFileSync(`tests/smoke.sh`, "x")'
CASES
miss=
for c in "${cases[@]}"; do pt_deny "$c" || miss="$miss [${c%%$'\n'*}]"; done
[ -z "$miss" ] && ok "検査のシェルへ書く呼び出し ${#cases[@]} 件（出来事の形6つと、sed -i・perl -i・リダイレクト・tee・python・node・perl の書き込み）をフックが deny し、Edit ツールを使う理由の1行を返す" \
  || ng "検査のシェルへ書く呼び出しを deny しない:$miss"
# 断る形が空でないこと（読み損ねると何も見ずに通る）
[ "${#cases[@]}" -ge 20 ] && ok "断る形を ${#cases[@]} 件読んだ" || ng "断る形を ${#cases[@]} 件しか読んでいない"

# 通す: 読むだけ・走らせるだけ・別の置き場への写し・検査のシェルでないファイルへの書き込み
read_cases <<'CASES'
bash tests/andon-digest.sh
%%
andon issue record A-0001 --expect fail -- bash tests/andon-digest.sh 2>&1 | tail -5
%%
for t in andon-digest process package; do bash tests/$t.sh >"$S/t-$t.log" 2>&1; echo "$t rc=$?"; done
%%
bash tests/smoke.sh > "$S/out.log" 2>&1
%%
grep -n "kind: scope" tests/andon-digest.sh; bash -n tests/schema.sh && bash -n tests/andon-digest.sh
%%
sed -n 1,20p tests/package.sh
%%
wc -l < tests/smoke.sh; cat tests/smoke.sh | grep -c ok > "$S/hits"
%%
git diff origin/main...HEAD -- tests/
%%
git add tests/smoke.sh && git commit -q -m 'x'
%%
cp tests/andon-digest.sh "$S/mut-base/tests/andon-digest.sh"
%%
sed -i 's/節 20/節 21/g' $S/pr-body.md && bash tests/package.sh
%%
python3 -c "print(open('tests/words.sh').read().count('ok'))"
%%
cat >> "$j/journal.md" <<'EOF'
- 作業: tests/*.sh を heredoc で包んだ python と sed -i で差し替えた。以後は Edit で直す。
EOF
%%
s=$S
cat > "$s/variants.py" <<'EOF'
import subprocess, sys, os
src, d = sys.argv[1], sys.argv[2]
sh = ['bash', 'tests/andon-digest.sh']
open(os.path.join(d, 'internal/digest/decided.go'), 'w').write('x')
subprocess.run(sh, cwd=d)
EOF
%%
python3 - "$m" <<'EOF'
import sys
p = sys.argv[1] + '/tests/andon-create.sh'
s = open(p).read()
open(p, 'w').write(s.replace('a', 'b'))
EOF
%%
python3 - "$S/pr.md" <<'EOF'
import sys
p = sys.argv[1]
s = open(p).read()
open(p, 'w').write(s.replace('`tests/andon-digest.sh` が通った', '`tests/andon-digest.sh` と `tests/smoke.sh` が通った'))
EOF
%%
grep -n '>' tests/smoke.sh
%%
grep -c ">>" tests/smoke.sh; grep -F '&>' tests/andon-boot.sh
%%
rg -n '>|' tests/*.sh
%%
python3 - <<'EOF'
s = open('tests/smoke.sh').read()
open('/tmp/s/out.txt', 'w').write(s)
EOF
%%
python3 -c "import sys; sys.stdout.write(open('tests/words.sh').read()[:200])"
%%
cd tests && grep -n ok smoke.sh && bash smoke.sh > "$S/o" 2>&1
%%
bash -c 'bash tests/smoke.sh > "$S/o" 2>&1'
%%
python3 - "$S" <<'EOF'
import json, sys
d = {'waived': [{'severity': 'trivial', 'file': 'tests/andon-digest.sh', 'line': 139}]}
open(sys.argv[1] + '/data.json', 'w').write(json.dumps(d))
EOF
%%
python3 - <<'EOF'
p = 'internal/ghpr/ghpr_test.go'
s = open(p).read()
open(p, 'w').write(s + '{"checks": {"state": "failure", "failed": ["tests"]}}\n')
EOF
%%
cat > "$S/mut.py" <<'EOF'
import os, subprocess, sys
d = sys.argv[1]
open(os.path.join(d, 'internal/digest/digest.go'), 'w').write('x')
subprocess.run(['bash', f'tests/{sys.argv[2]}.sh'], cwd=d)
EOF
%%
cd $S/old && sed -i 's/a/b/' tests/andon-dispatcher.sh
%%
echo "$(bash tests/smoke.sh 2>&1 | tail -1)"; n="$(grep -c '>' tests/smoke.sh)"
%%
node -e 'console.log(require("fs").readFileSync(`tests/smoke.sh`, "utf8").length)'
CASES
extra=
for c in "${cases[@]}"; do out=$(pt Bash "$c"); [ -z "$out" ] || extra="$extra [${c%%$'\n'*}]"; done
[ -z "$extra" ] && ok "読むだけ・走らせるだけ・別の置き場への写し・ほかのファイルへの書き込みの呼び出し ${#cases[@]} 件はフックが通す" \
  || ng "フックが通すはずの呼び出しを断る:$extra"

# cwd が tests/ のとき（前の呼び出しで cd tests した）は、tests/ の字が無くても検査のシェルを指す。CLAUDE_PROJECT_DIR が無ければ cwd を根と見る
cwd_out() { # cwd_out <cwd> <command> [<CLAUDE_PROJECT_DIR>]
  jq -n --arg c "$2" --arg d "$1" '{tool_name: "Bash", cwd: $d, tool_input: {command: $c}}' | CLAUDE_PROJECT_DIR=${3-$root} "$h" 2>/dev/null || true
}
extra=
[ -n "$(cwd_out "$root/tests" "sed -i 's/a/b/' smoke.sh")" ] || extra="$extra [cwd tests の sed -i]"
[ -n "$(cwd_out "$root/tests" "echo x >> lib/isolate.sh")" ] || extra="$extra [cwd tests のリダイレクト]"
[ -z "$(cwd_out "$root/tests" "grep -n ok smoke.sh; bash smoke.sh > \"\$S/o\" 2>&1")" ] || extra="$extra [cwd tests の読むだけ]"
[ -z "$(cwd_out "$root/docs" "sed -i 's/a/b/' tests/x.sh")" ] || extra="$extra [cwd docs の docs/tests/x.sh]"
[ -n "$(cwd_out "$root" "sed -i 's/a/b/' $root/tests/smoke.sh" "")" ] || extra="$extra [CLAUDE_PROJECT_DIR が無いときの根の絶対パス]"
[ -z "$extra" ] && ok "cwd が tests/ の下なら tests/ の字が無くても断り、読むだけは通し、根の外の tests/ は見ない" || ng "cwd を辿り損ねる:$extra"

# フックが走れないときは止める側に倒す: python3 が無ければ settings の command が tests を含む呼び出しを終了コード 2 で止め、
# フックの中で例外が出れば tests を含む呼び出しを deny する。tests を含まない呼び出しは、どちらでも通す
extra=
mkdir -p "$tmp/nopy"; ln -s "$(command -v grep)" "$tmp/nopy/grep"
rc=0; input Bash "sed -i 's/a/b/' tests/smoke.sh" | env -i PATH="$tmp/nopy" CLAUDE_PROJECT_DIR="$root" /bin/sh -c "$hook_cmd" >/dev/null 2>"$tmp/nopy.err" || rc=$?
{ [ "$rc" = 2 ] && grep -q 'python3' "$tmp/nopy.err"; } || extra="$extra [python3 が無いとき tests を含む呼び出しを止めない（$rc）]"
rc=0; input Bash "git status" | env -i PATH="$tmp/nopy" CLAUDE_PROJECT_DIR="$root" /bin/sh -c "$hook_cmd" >/dev/null 2>&1 || rc=$?
[ "$rc" = 0 ] || extra="$extra [python3 が無いとき tests を含まない呼び出しを止める（$rc）]"
# 例外を出す写し（解析の入口で投げる）
sed 's/^def denied(text, cur, whole, depth=0):$/&\n    raise RuntimeError("壊した写し")/' "$h" >"$tmp/broken" 2>/dev/null || true
grep -q '壊した写し' "$tmp/broken" || extra="$extra [例外を出す写しを作れない]"
out=$(input Bash "bash tests/smoke.sh" | CLAUDE_PROJECT_DIR=$root python3 "$tmp/broken" 2>/dev/null || true)
jq -e '.hookSpecificOutput.permissionDecision == "deny"' <<<"$out" >/dev/null 2>&1 || extra="$extra [例外のとき tests を含む呼び出しを deny しない]"
out=$(input Bash "git status" | CLAUDE_PROJECT_DIR=$root python3 "$tmp/broken" 2>/dev/null || true)
[ -z "$out" ] || extra="$extra [例外のとき tests を含まない呼び出しまで deny する]"
[ -z "$extra" ] && ok "python3 が無いときとフックの中の例外では、tests を含む呼び出しを止め、含まない呼び出しは通す" || ng "フックが走れないときに止める側に倒れない:$extra"

# Bash でないツール・壊れた入力は、何も出さずに 0 で通す
extra=
out=$(pt Edit "sed -i s/a/b/ tests/x.sh"); [ -z "$out" ] || extra="$extra [Edit]"
rc=0; out=$(printf 'not json' | "$h" 2>/dev/null) || rc=$?; { [ -z "$out" ] && [ "$rc" = 0 ]; } || extra="$extra [壊れた入力]"
rc=0; out=$(jq -n '{tool_name: "Bash", tool_input: {command: 7}}' | "$h" 2>/dev/null) || rc=$?; { [ -z "$out" ] && [ "$rc" = 0 ]; } || extra="$extra [command が文字列でない]"
[ -z "$extra" ] && ok "Bash でないツール・壊れた入力はフックが何も出さずに通す" || ng "通すはずの入力で何か出すか 0 で終わらない:$extra"

exit "$fail"
