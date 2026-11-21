# sandbox.sh: journeys/*.sh が source する。切り離した場所を作り、セルに入っている本物の andon・`kanban`・`/opt/andon` で動かす。
#   - drive.sh が渡す VERIFY_TMP の下だけに書く。HOME・kanban・git の設定・状態の置き場を一時へ向ける（tests/lib/isolate.sh）。
#   - andon と kanban は PATH にある本物（当てたもの）。ANDON_HOME は外すので、パッケージは既定の /opt/andon（当てたもの）。
#   - 偽の hc と gh を PATH の先頭に置く。本物の端末と GitHub には届かない。呼ばれた引数は $tmp/hc.calls・$tmp/gh.calls に残る。
#     偽の hc: get <addr> --json は $tmp/hc/<addr>.json を返す（無ければ 1）。ほかは記録して 0。
#     偽の gh: pr view <url> --json headRefOid -q .headRefOid は $tmp/gh/head、--json state -q .state は OPEN。ほかは記録して 99。
# 読んだあとに使えるもの: common.sh の分と $andon・$kanban・ledger_init <prefix>
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
andon=$(command -v andon) kanban=$(command -v kanban)
[ -n "$andon" ] && [ -n "$kanban" ] || { echo "FAIL andon か kanban が PATH に無い"; exit 1; }
. "${VERIFY_REPO:?}/tests/lib/isolate.sh"
unset ANDON_HOME
export HOME="$tmp/home" GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=verify GIT_AUTHOR_EMAIL=verify@example.com GIT_COMMITTER_NAME=verify GIT_COMMITTER_EMAIL=verify@example.com
mkdir -p "$HOME" "$tmp/bin" "$tmp/hc" "$tmp/gh"
cat >"$tmp/bin/hc" <<EOF
#!/bin/sh
echo "hc \$*" >>"$tmp/hc.calls"
if [ "\$1" = get ] && [ "\$3" = --json ]; then cat "$tmp/hc/\$2.json" 2>/dev/null || exit 1; fi
exit 0
EOF
cat >"$tmp/bin/gh" <<EOF
#!/bin/sh
echo "gh \$*" >>"$tmp/gh.calls"
case "\$1 \$2 \$4 \$5 \$6" in
  "pr view --json headRefOid -q") cat "$tmp/gh/head" ;;
  "pr view --json state -q") echo OPEN ;;
  *) echo "偽の gh: 知らない呼び出し \$*" >&2; exit 99 ;;
esac
EOF
chmod +x "$tmp/bin/hc" "$tmp/bin/gh"
export PATH="$tmp/bin:$PATH"

# ledger_init <prefix>: $tmp/project に kanban を作り、KANBAN_ROOT を向ける。Chief of Staff は端末 tc
ledger_init() {
  mkdir -p "$tmp/project" && cd "$tmp/project" || exit 1
  "$kanban" init --prefix "$1" >/dev/null && export KANBAN_ROOT=$tmp/project
  HC_TERMINAL_ID=tc "$kanban" config set chief_of_staff=tc >/dev/null
}
