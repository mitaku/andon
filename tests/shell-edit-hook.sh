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

# フックの在りかと繋ぎ方
if jq -e '.hooks.PreToolUse | any(.[]; .matcher == "Bash" and any(.hooks[]; .type == "command" and .command == "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/test-shell-edit"))' .claude/settings.json >/dev/null 2>&1 \
  && [ -x "$h" ] && python3 -c 'import ast, sys; ast.parse(open(sys.argv[1]).read())' "$h" 2>/dev/null; then
  ok ".claude/settings.json の PreToolUse（matcher Bash）が .claude/hooks/test-shell-edit を指し、フックは実行できる"
else
  ng ".claude/settings.json に Bash の PreToolUse のフック（.claude/hooks/test-shell-edit）が無いか、フックが実行できない"
fi

# pt <tool> <command>: フックに PreToolUse の入力を渡し、出力を返す（フックが無ければ空）
pt() {
  jq -n --arg t "$1" --arg c "$2" --arg d "$root" '{hook_event_name: "PreToolUse", tool_name: $t, cwd: $d, tool_input: {command: $c}}' \
    | CLAUDE_PROJECT_DIR=$root "$h" 2>/dev/null || true
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
cd /workspace/.worktrees/andon/andon-A-0152; python3 - <<'PYEND'
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
s=/tmp/claude-1000/x/scratchpad
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
cd /workspace/.worktrees/andon/andon-A-0328
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
cat >> /workspace/projects/p/kanban/issues/X-0001/library/journal.md <<'EOF'
- 作業: tests/*.sh を heredoc で包んだ python と sed -i で差し替えた。以後は Edit で直す。
EOF
%%
s=/tmp/claude-1000/x/scratchpad
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
CASES
extra=
for c in "${cases[@]}"; do out=$(pt Bash "$c"); [ -z "$out" ] || extra="$extra [${c%%$'\n'*}]"; done
[ -z "$extra" ] && ok "読むだけ・走らせるだけ・別の置き場への写し・ほかのファイルへの書き込みの呼び出し ${#cases[@]} 件はフックが通す" \
  || ng "フックが通すはずの呼び出しを断る:$extra"

# Bash でないツール・壊れた入力は、何も出さずに 0 で通す
extra=
out=$(pt Edit "sed -i s/a/b/ tests/x.sh"); [ -z "$out" ] || extra="$extra [Edit]"
rc=0; out=$(printf 'not json' | "$h" 2>/dev/null) || rc=$?; { [ -z "$out" ] && [ "$rc" = 0 ]; } || extra="$extra [壊れた入力]"
rc=0; out=$(jq -n '{tool_name: "Bash", tool_input: {command: 7}}' | "$h" 2>/dev/null) || rc=$?; { [ -z "$out" ] && [ "$rc" = 0 ]; } || extra="$extra [command が文字列でない]"
[ -z "$extra" ] && ok "Bash でないツール・壊れた入力はフックが何も出さずに通す" || ng "通すはずの入力で何か出すか 0 で終わらない:$extra"

exit "$fail"
