# tests/*.sh が（tests/lib/andon.sh・kanban.sh から）source する: 検査をセルの本物の kanban から切り離す。
#   - 一時ディレクトリの根（$tmp の親。TMPDIR か /tmp）のさらに親を GIT_CEILING_DIRECTORIES に足す。git は一時ディレクトリ
#     （と根そのもの）から上の repo を探さない（ceiling の dir そのものからは上を探すので、根ではなくその親を置く）。
#     TMPDIR が repo の中（担当の worktree の中など）でも、kanban の CLI の探し方（git の main checkout → その上の projects/*/kanban）が
#     セルの本物の kanban に届かない。一時ディレクトリの中に作った repo はこれまでどおり見つかる。
#   - 受け継いだ KANBAN_ROOT を外す（kanban は一時ディレクトリの中のものだけ。要る検査はこのあとで自分の kanban を指す）。
#   - HOME より先に読まれる置き場（CLAUDE_CONFIG_DIR・CODEX_HOME・ANDON_HOME・XDG_STATE_HOME・XDG_CONFIG_HOME・XDG_DATA_HOME）と
#     hc の端末（HC_*）を外す。検査が HOME を一時に向ければ、置き場もその下になる。要る検査はこのあとで一時の置き場を指す。
#   - セルの repo（ANDON_WORKSPACE）と clone の根（GHQ_ROOT）は、外すと既定（/workspace・ghq root）がセルの本物を指すので、
#     一時ディレクトリの中の無い場所へ向ける。要る検査はこのあとで自分の置き場を指す。
#   - 報告を集めて読む場所へ送るのを切る（ANDON_TRAY_OFF。ADR-0064 決めたこと 15）。andon file publish と andon digest send は
#     送らずに files/ へ写し、andon issue report は kanban にだけ書く。セルで動いている本物の送り先に検査の報告を届けない。
#     image/apply-main.sh もその場所を組まない（組む道具は検査の環境に無いことがあり、その検査は別の CI の job が回す）。
# 呼ぶ側は $tmp（後片付けする一時ディレクトリ。mktemp -d）を先に作っておく。
ceiling=$(cd "$(dirname "$tmp")/.." && pwd -P)
case ":${GIT_CEILING_DIRECTORIES:-}:" in
  *":$ceiling:"*) ;;
  *) export GIT_CEILING_DIRECTORIES="$ceiling${GIT_CEILING_DIRECTORIES:+:$GIT_CEILING_DIRECTORIES}" ;;
esac
unset KANBAN_ROOT CLAUDE_CONFIG_DIR CODEX_HOME ANDON_HOME XDG_STATE_HOME XDG_CONFIG_HOME XDG_DATA_HOME HC_TERMINAL_ID HC_TERMINAL_SOCKET HC_SCOPE
export ANDON_WORKSPACE="$tmp/.isolate/workspace" GHQ_ROOT="$tmp/.isolate/ghq" ANDON_TRAY_OFF=1
