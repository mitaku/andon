#!/usr/bin/env bash
# tests/lib/scope.sh [--heavy] < 変更されたパス（repo からの相対・1行1つ）
#
# 手元で「変えたものに関係する検査だけ」を走らせるとき（tests/run --changed）と CI の文書だけの PR で、走らせる tests/*.sh を
# 1行1つで出す。安く全体を見るもの（public・public-selftest・smoke・ci・words・words-selftest・agents-md・verify-andon・shell-and）はいつも。コードや package/ を
# 変えたら、重い検査（下の表）を除く全部と、変えたパスが「見る部分」に入る重い検査を足す。知らないパスは重い検査を含む全部。
# tests/image.sh は CI と同じ tests/lib/image-scope.sh が true のときだけ。表は docs/image.md の「手元は関係する検査だけ」。
#   --heavy  重い検査と見る部分の表を「<検査> <部分>」で1行ずつ出す（tests/ci.sh が Go の import と突き合わせる）
set -euo pipefail
cd "$(dirname "$0")/../.."

# 重い検査と、その検査が見る部分（ADR-0032 決めたこと 4）。「<dir>/」はその下の全部、それ以外はそのパス（* は同じ段の中だけ）。
# Go の package は、検査が打つ command の package と、それが import する package（go list -deps）。embed.go は root の package で、
# init/ と process/states/issue.yaml と schema/ の直下の *.schema.json（書く前に照らす。internal/contract が読む）を埋め込む。cmd/andon と internal/cli は全部の command の入口で、どの検査の部分にも入る
# （import を辿ると全部の package になるので、tests/ci.sh は閉じているかの確かめから除く）。
go_issue='embed.go init/ process/states/issue.yaml schema/*.schema.json cmd/andon/ cmd/kanban/ internal/cli/
  internal/issue/ internal/dispatcher/ internal/gate/ internal/boot/ internal/chiefengineer/ internal/process/
  internal/triage/ internal/retro/ internal/ghpr/ internal/signal/ internal/launch/ internal/sessionlog/ internal/pkgversion/
  internal/ledger/ internal/flock/ internal/delegate/ internal/spawn/ internal/contract/ internal/redact/
  internal/waiting/'
declare -A heavy=(
  # andon dispatcher・andon issue・andon gate・kanban を打つ。ANDON_HOME を package/ に向け、gate.json とフックを読む
  [tests/andon-dispatcher.sh]="$go_issue package/gate.json package/hooks/"
  # andon issue・andon chief-engineer・kanban を打つ。ANDON_HOME を package/ に向け、gate.json を読む（review: off の課題のレベル）
  [tests/andon-issue.sh]="$go_issue package/gate.json"
  # andon gate・andon delegate・andon issue・kanban を打つ。ANDON_HOME を package/ に向け、gate.json を読む
  [tests/andon-gate.sh]="$go_issue package/gate.json"
  # image/apply-main.sh を当て、焼いた andon の help・version と kanban --version を打つ
  [tests/apply-main.sh]="image/apply-main.sh cmd/andon/ cmd/kanban/ internal/cli/ internal/pkgversion/
    internal/ledger/ internal/delegate/ internal/spawn/ internal/contract/ embed.go"
  # schema/ の直下と contract.md の例を検査器（andon schema。lint は schema_check.py）で見る。command の出力は emitters が
  # 写したものを読む（下で emitters も足す）
  [tests/schema.sh]="schema/*.schema.json schema/ledger/*.schema.json docs/contract.md package/lib/andon/schema_check.py
    cmd/andon/ internal/cli/ internal/schema/ internal/contract/ embed.go"
)

# parts <検査>: 見る部分を配列 parts に読む（* を今のファイルに広げない）
parts() { read -r -d '' -a parts <<<"${heavy[$1]}" || true; }

if [ "${1:-}" = --heavy ]; then
  for t in "${!heavy[@]}"; do parts "$t"; for part in "${parts[@]}"; do echo "$t $part"; done; done | sort
  exit 0
fi

# sees <部分> <パス>: パスが部分に入るか
sees() {
  case "$1" in
    */) [[ "$2" == "$1"* ]] ;;
    *'*'*) [[ "$2" == $1 && "${2%/*}" == "${1%/*}" ]] ;;
    *) [ "$2" = "$1" ] ;;
  esac
}

# agents-md.sh は AGENTS.md の「決まり｜守らせるもの」の表を見る。AGENTS.md だけの PR でも走るよう、いつもの側に置く（ADR-0035 決めたこと 6）。
# verify-andon.sh は検証の skill の目次・地図・名指す command の実在を見る。名指す command を消すコードの PR でも、skill の文だけの PR でも
# 走るよう、いつもの側に置く（ADR-0035 決めたこと 5）。
# shell-and.sh は tests/ の下のシェルの、set -e で止まらない A && B の判定を見る。検査の1本だけを書き換えた PR でも走るよう、いつもの側に置く
declare -A run=([tests/public.sh]=1 [tests/public-selftest.sh]=1 [tests/smoke.sh]=1 [tests/ci.sh]=1 [tests/words.sh]=1 [tests/words-selftest.sh]=1 [tests/agents-md.sh]=1 [tests/verify-andon.sh]=1 [tests/shell-and.sh]=1)
light=false # 重い検査を除く全部
all=false   # 重い検査も含む全部（知らないパス）
paths=()
while IFS= read -r p; do
  [ -n "$p" ] || continue
  paths+=("$p")
  # root の Go のテスト（workflow_test.go など。image に入らない）
  case "$p" in */*) ;; *_test.go) continue ;; esac
  # 重い検査は、変えたパスがその検査の見る部分に入るときだけ（Go のテストは go test ./... が見る）
  case "$p" in cmd/*_test.go | internal/*_test.go) ;; *)
    for t in "${!heavy[@]}"; do
      parts "$t"
      for part in "${parts[@]}"; do sees "$part" "$p" && run[$t]=1 && break; done
    done ;;
  esac
  # kanban.sh は schema/ledger・docs/ledger.md・cmd/kanban が指す docs/ の文書があるかを見る。消した・動かした文書で足す
  case "$p" in docs/*) [ -e "$p" ] || run[tests/kanban.sh]=1 ;; esac
  case "$p" in
    # contract.md は §4 の acceptance[] の行を skill と突き合わせる（package.sh）。例と schema は schema.sh（上の表）
    docs/contract.md) run[tests/package.sh]=1 ;;
    # design.md は §1.8 の文書の形と役の表を CE skill と突き合わせ、語の検査の範囲にも入る（package.sh）
    docs/design.md) run[tests/package.sh]=1 ;;
    docs/ledger.md) run[tests/kanban.sh]=1 ;;
    # commands.md は andon の葉の表から作る一覧（ADR-0053 決めたこと 16）。作り直しても変わらないかを commands.sh が見る
    docs/commands.md) run[tests/commands.sh]=1 ;;
    # adr-index.sh は ADR の索引・状態行（ADR-0033）、ADR ごとの決定の記録（docs/decisions/・decisions.md §14）、
    # docs/adr/README.md の一覧の見出しのバージョンと CHANGELOG.md の一番上のバージョン（ADR-0055）を突き合わせる
    docs/adr/*) run[tests/package.sh]=1 run[tests/adr-index.sh]=1 ;;
    docs/decisions.md | docs/decisions/* | CHANGELOG.md) run[tests/adr-index.sh]=1 ;;
    tests/kanban/*) run[tests/kanban.sh]=1 ;;
    # ADR の番号を取る command（ADR-0055 決めたこと 6）。検査ではないので名前に .sh を付けない。打つのは adr-new.sh
    tests/adr-new) run[tests/adr-new.sh]=1 ;;
    # package.sh の節（1節1ファイル）。入口の tests/package.sh が順に読む
    tests/package/*) run[tests/package.sh]=1 ;;
    tests/host/*) run[tests/host.sh]=1 ;;
    # 流れと状態の定義（ADR-0017）。dispatcher.md §7.2 は once: true の移り方と突き合わせる。
    # process/states/issue.yaml は andon に埋め込む（embed.go）ので、軽い検査も全部（重い検査は上の表）
    process/states/issue.yaml) run[tests/process.sh]=1 light=true ;;
    process/* | schema/process/* | docs/process/* | tests/process/* | docs/dispatcher.md) run[tests/process.sh]=1 ;;
    tests/lib/pty-probe.py | tests/image.sh) ;; # image-scope が見る
    # セルに main の commit を当てる道具（ADR-0027）。image に入らない。apply-main.sh は上の表
    image/apply-main.sh) ;;
    # 判定の道具は tests/ci.sh（いつも）が見る。image-scope.sh は image.sh も（image-scope が見る）
    tests/lib/scope.sh | tests/lib/docs-scope.sh | tests/lib/image-scope.sh) ;;
    tests/lib/*) all=true ;;
    tests/*.sh) [ -f "$p" ] && run[$p]=1 ;;
    # repo の中の skill の置き場（ADR-0035 決めたこと 4）。image にも andon にも入らない。形は verify-andon.sh、語は words.sh（どちらもいつもの分）
    .claude/skills/* | .agents/skills/*) ;;
    # 検査のシェルを Bash で書き換える呼び出しを断る、この repo の Claude Code のフック。image にも andon にも入らない。.claude/settings.json は知らないパスのまま
    .claude/hooks/test-shell-edit) run[tests/shell-edit-hook.sh]=1 ;;
    # yomiyasu の許可の一覧。image にも andon にも入らない。正との突き合わせは package.sh の節（tests/package/23-yomiyasu-words.sh）、語は words.sh（いつもの分）
    .yomiyasu-words/*) run[tests/package.sh]=1 ;;
    # Go のテスト（go test ./... が見る）・人用の文書は、いつもの分だけ
    cmd/*_test.go | internal/*_test.go | docs/* | tests/* | README.md | AGENTS.md | REVIEW.md | SECURITY.md | LICENSE) ;;
    # コード・andon に埋め込むもの・image に入るもの。重い検査は上の表で足した分だけ
    cmd/* | internal/* | package/* | schema/* | init/* | image/* | embed.go) light=true ;;
    # go.mod・go.sum・mise.toml・.dockerignore・.github/・model/ とまだ無いパス
    *) all=true ;;
  esac
done
if [ "$all" = true ]; then
  for f in tests/*.sh; do run[$f]=1; done
elif [ "$light" = true ]; then
  for f in tests/*.sh; do [ -n "${heavy[$f]:-}" ] || run[$f]=1; done
fi
# schema.sh は emitters が同じ回に写したものを読む。並べて走らせないと、schema.sh が自分の中で1本ずつ走らせて遅い
if [ -n "${run[tests/schema.sh]:-}" ]; then
  while IFS= read -r e; do run[$e]=1; done < <(bash tests/schema.sh --emitters)
fi
unset 'run[tests/image.sh]'
[ "$(printf '%s\n' "${paths[@]+"${paths[@]}"}" | bash tests/lib/image-scope.sh)" = true ] && run[tests/image.sh]=1
for f in tests/*.sh; do [ -n "${run[$f]:-}" ] && echo "$f"; done
exit 0
