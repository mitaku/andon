#!/usr/bin/env bash
# image/apply-main.sh apply [<commit>] | status | revert
#
# Andon を開発するセルで、origin/main の commit をイメージの焼き直しなしにセルへ当てる（ADR-0027）。
#   apply [<commit>]  commit（既定は origin/main の先頭。origin/main に含まれるものだけ）の package/・schema/ を
#                     /opt/andon に、そこから焼いた andon・kanban を置き、andon boot を打ち直して dispatcher を起動し直す。
#                     最初に当てる前に、イメージの /opt/andon と kanban を退避する。
#   status            イメージの版・当てた commit・origin/main の先頭からの遅れを出す。
#                     イメージの元の commit から当てた commit（origin/main の先頭）までに image/ が変わっていたら警告も出す。
#   revert            退避からイメージの版へ戻し、記録を消し、andon boot を打ち直して dispatcher を起動し直す。
# 書く先と元の repo は環境変数で差し替える（検査が一時の置き場に向ける）:
#   ANDON_APPLY_REPO（既定はこのスクリプトの repo）・ANDON_APPLY_ROOT（/opt/andon）・ANDON_APPLY_KANBAN（/usr/local/bin/kanban）・
#   ANDON_APPLY_STASH（退避。/opt/andon-image）・ANDON_APPLY_ANDON（boot を打つ andon。既定は <ROOT>/bin/andon）。
# 書く先の親に書けなければ sudo で書く。andon の持ち物の置き場なら sudo は使わない。
set -euo pipefail

usage() { sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }
die() { echo "✗ $*" >&2; exit 1; }
say() { echo "[apply-main] $*"; }

[ "$#" -ge 1 ] || usage
cmd=$1
shift

repo=${ANDON_APPLY_REPO:-$(cd "$(dirname "$0")/.." && pwd)}
root=${ANDON_APPLY_ROOT:-/opt/andon}
kanban=${ANDON_APPLY_KANBAN:-/usr/local/bin/kanban}
stash=${ANDON_APPLY_STASH:-/opt/andon-image}
record=$root/applied.json # 当てた commit・日時・イメージの元の commit。/opt/andon の中なので作り直しで一緒に消える
git -C "$repo" rev-parse --git-dir >/dev/null 2>&1 || die "$repo が git の repo でない（ANDON_APPLY_REPO で andon の checkout を指す）"

# 書く先の親に書けなければ sudo（/opt と /usr/local/bin は root の持ち物）
as_root=
for d in "$(dirname "$root")" "$(dirname "$kanban")" "$(dirname "$stash")"; do
  [ -w "$d" ] || as_root=sudo
done
[ ! -e "$root" ] || [ -w "$root" ] || as_root=sudo

# 同時に2つ走らせない（退避の隣の鍵。読むだけで開いて flock）
lock() {
  local f=$stash.lock
  [ -e "$f" ] || $as_root touch "$f"
  exec 8<"$f"
  flock -n 8 || die "ほかの apply-main.sh が動いている（$f）"
}

short() { printf '%s' "${1:0:12}"; }

# 当てる前から動いている端末（古い skill のまま動いているもの）の一覧。止めない。dispatcher は道具が起動し直すので除く
live_terminals() {
  command -v hc >/dev/null || return 0
  env -u HC_TERMINAL_ID hc ls --json 2>/dev/null 8<&- 9<&- \
    | jq -r '.terminals[]? | select(.state == "live" and .name != "dispatcher") | "  \(.address) \(.name // "-") \(.status // "-")"' 2>/dev/null || true
}

# イメージの元の commit($1)から $2 までに image/ が変わったファイルを並べて警告する（apply-main.sh 自身はイメージに入らないので除く）
warn_image_drift() {
  local from=$1 to=$2 files
  git -C "$repo" rev-parse --verify --quiet "$from^{commit}" >/dev/null 2>&1 || return 0
  files=$(git -C "$repo" diff --name-only "$from" "$to" -- image ':!image/apply-main.sh' 2>/dev/null) || return 0
  [ -n "$files" ] || return 0
  say "警告: イメージの元の commit（$(short "$from")）から $(short "$to") までに image/ が変わっている。焼き直しが要るかもしれない（aqua の道具・Claude Code・bash の設定などはこの道具では変わらない）"
  printf '%s\n' "$files" | sed 's/^/  image変更: /'
}

fetch() { git -C "$repo" fetch --quiet origin 2>/dev/null || say "警告: git fetch origin に失敗した。手元の origin/main で続ける"; }

main_head() { git -C "$repo" rev-parse --verify --quiet 'origin/main^{commit}' || die "$repo に origin/main が無い"; }

# イメージの版: 退避があればそこに控えた値、無ければ今の kanban --version
image_commit() {
  if [ -f "$stash/image-commit" ]; then
    cat "$stash/image-commit"
  else
    local v
    v=$("$kanban" --version 2>/dev/null | awk '{print $2}') || true
    printf '%s' "${v:-unknown}"
  fi
}

applied_commit() { [ -f "$record" ] && sed -n 's/.*"commit": *"\([0-9a-f]*\)".*/\1/p' "$record" | head -1 || true; }

# $1 を $2 に入れ替える（同じファイルシステムの中の rename）。overlay の下の層にある dir（イメージの /opt/andon）は
# rename できない（EXDEV）ので、そのときは消してから入れる（中身は退避にある）。入れられなければ退避から戻す。
swap_dir() {
  local new=$1 target=$2 old=
  if [ -e "$target" ]; then
    if $as_root mv -T --no-copy "$target" "$target.old.$$" 2>/dev/null; then
      old=$target.old.$$
    else
      $as_root rm -rf "$target"
    fi
  fi
  if ! $as_root mv -T --no-copy "$new" "$target"; then
    if [ -n "$old" ]; then
      $as_root mv -T --no-copy "$old" "$target"
    else
      $as_root cp -a "$stash/andon" "$target"
    fi
    $as_root rm -rf "$new"
    die "$target を入れ替えられなかった（元に戻した）"
  fi
  [ -z "$old" ] || $as_root rm -rf "$old"
}

# andon boot を打ち直し、dispatcher を起動し直す。dispatcher の端末を片付けてから boot（boot が無い端末を起動する）。
# 見回りの途中で止めないよう、dispatcher の見回りの鍵（internal/dispatcher の PassLock）が外れるのを待つ。
# 鍵を取るのは、見回りのたびに鍵を取る版の dispatcher だけ。前の版は鍵を取らないので、鍵が取れても見回りの合間とは限らない。
# 版は、動いている dispatcher の実体（/proc/<pid>/exe。入れ替えたあとも動き始めたときの中身）に鍵のファイル名があるかで見る
# （鍵のファイルがあるかでは見ない。戻したあとや作り直したあとにも、前に作られたファイルが残る）。
# 確かめられないときは止めない
reboot() {
  local andon=${ANDON_APPLY_ANDON:-$root/bin/andon}
  local passlock=${XDG_STATE_HOME:-$HOME/.local/state}/andon/dispatcher/.pass.lock
  local got addr= pid=
  if ! command -v hc >/dev/null; then
    say "hc が無いので dispatcher は触らない"
  elif got=$(env -u HC_TERMINAL_ID hc get dispatcher --json 2>/dev/null 8<&-); then
    addr=$(jq -r '.address // empty' <<<"$got" 2>/dev/null || true)
    pid=$(jq -r '.foreground.pid // empty' <<<"$got" 2>/dev/null || true)
  fi
  if [ -n "$addr" ] && ! { [ -n "$pid" ] && grep -aq '\.pass\.lock' "/proc/$pid/exe" 2>/dev/null; }; then
    say "警告: 動いている dispatcher（$addr）が見回りの鍵を取る版と確かめられないので、止めずに残した（見回りの途中で止めない）。"
    say "  新しい andon で動かすには、見回りの合間に hc kill $addr・hc rm $addr を打ってから $andon boot を打つ"
    addr=
  elif [ -n "$addr" ]; then
    # 鍵を取る版は見回りのたびにこのファイルを作るので、ここで作ってもよい
    mkdir -p "$(dirname "$passlock")"
    exec 9>>"$passlock"
    if ! flock -w 900 9; then
      say "警告: dispatcher の見回りが15分終わらないので起動し直さなかった。終わってから $andon boot を打ち、hc で dispatcher を起動し直す"
      return 1
    fi
  fi
  if [ -n "$addr" ]; then
    for a in "keep $addr --off" "kill $addr" "rm $addr"; do
      # shellcheck disable=SC2086
      env -u HC_TERMINAL_ID hc $a >/dev/null 2>&1 8<&- 9>&- || say "警告: hc $a に失敗した"
    done
    say "dispatcher の端末（$addr）を片付けた"
  fi
  local rc=0
  # 鍵の fd は子に渡さない（boot が起動する常駐のものが持ち続けると、見回りが止まる）
  ANDON_HOME=$root "$andon" boot 8<&- 9>&- || rc=$?
  exec 9>&-
  [ "$rc" = 0 ] || { say "警告: andon boot が rc $rc で終わった（出力を見て打ち直す）"; return 1; }
  say "andon boot を打ち直した（dispatcher が無ければ boot が起動する）"
}

gobin() {
  if command -v go >/dev/null; then
    command -v go
  elif command -v mise >/dev/null; then
    (cd "$repo" && mise which go) || die "go が無い（mise の go も取れない）"
  else
    die "go が無い"
  fi
}

apply() {
  [ "$#" -le 1 ] || usage
  fetch
  local head commit
  head=$(main_head)
  commit=$(git -C "$repo" rev-parse --verify --quiet "${1:-$head}^{commit}") || die "commit が無い: ${1:-}"
  git -C "$repo" merge-base --is-ancestor "$commit" "$head" \
    || die "$commit は origin/main（$(short "$head")）に含まれない。当てられるのはマージ済みの commit だけ"

  # 当てる commit のスクリプトで当てる（中身と版を揃える。ADR-0027 決めたこと 5）
  if [ "${ANDON_APPLY_SELF:-}" != "$commit" ]; then
    local self
    self=$(mktemp)
    if git -C "$repo" show "$commit:image/apply-main.sh" >"$self" 2>/dev/null && ! cmp -s "$self" "$0"; then
      say "$(short "$commit") の image/apply-main.sh で当てる"
      ANDON_APPLY_SELF=$commit ANDON_APPLY_REPO=$repo ANDON_APPLY_SELF_FILE=$self exec bash "$self" apply "$commit"
    fi
    rm -f "$self"
  fi
  [ -z "${ANDON_APPLY_SELF_FILE:-}" ] || rm -f "$ANDON_APPLY_SELF_FILE"

  lock
  build=$(mktemp -d) # trap が EXIT で読むので local にしない
  trap 'rm -rf "${build:-}"' EXIT

  # 一時の場所で組む（ここで落ちたら、書く先には何も触っていない）
  say "$(short "$commit") から組む"
  mkdir -p "$build/src" "$build/pkg"
  git -C "$repo" archive "$commit" | tar -x -C "$build/src"
  cp -R "$build/src/package/." "$build/pkg/"
  cp -R "$build/src/schema" "$build/pkg/schema"
  mkdir -p "$build/pkg/bin"
  local go
  go=$(gobin)
  (cd "$build/src" \
    && CGO_ENABLED=0 "$go" build -trimpath -ldflags "-s -w -X main.version=$(short "$commit")" -o "$build/kanban" ./cmd/kanban \
    && CGO_ENABLED=0 "$go" build -trimpath -ldflags "-s -w" -o "$build/pkg/bin/andon" ./cmd/andon) \
    || die "go build に失敗した（何も入れ替えていない）"

  # 最初に当てる前に、イメージの /opt/andon と kanban を退避する
  local image terminals
  image=$(image_commit)
  terminals=$(live_terminals) # 当てる前から動いている端末
  warn_image_drift "$image" "$commit"
  if [ ! -d "$stash/andon" ]; then
    local s=$stash.new.$$
    $as_root rm -rf "$s"
    $as_root mkdir -p "$s"
    $as_root cp -a "$root" "$s/andon"
    $as_root cp -a "$kanban" "$s/kanban"
    printf '%s' "$image" | $as_root tee "$s/image-commit" >/dev/null
    $as_root rm -rf "$stash"
    $as_root mv -T --no-copy "$s" "$stash"
    say "イメージの版（$image）を $stash に退避した"
  fi

  printf '{"commit": "%s", "applied_at": "%s", "image_commit": "%s"}\n' \
    "$commit" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$image" >"$build/pkg/applied.json"

  # 入れ替える: kanban は隣に置いてから rename（動いている kanban を書き換えない。text file busy にならない）
  $as_root cp "$build/kanban" "$kanban.new.$$"
  $as_root chmod 755 "$kanban.new.$$"
  $as_root rm -rf "$root.new.$$"
  $as_root cp -R "$build/pkg" "$root.new.$$"
  swap_dir "$root.new.$$" "$root"
  $as_root mv -f -T --no-copy "$kanban.new.$$" "$kanban"
  say "$(short "$commit") を当てた（$root・$kanban）"
  if [ -n "$terminals" ]; then
    say "当てる前から動いている端末（止めていない。新しい skill は次に起動するエージェントから効く。起動し直すかは一覧を見て決める）:"
    printf '%s\n' "$terminals" >&2
  fi

  reboot || true
}

revert() {
  [ "$#" -eq 0 ] || usage
  if [ ! -d "$stash/andon" ]; then
    say "当てていない（退避 $stash が無い）。戻すものは無い"
    return 0
  fi
  lock
  # andon・kanban が壊れていても動くよう、ここでは cp・mv・rm だけを使う
  $as_root rm -rf "$root.new.$$"
  $as_root cp -a "$stash/andon" "$root.new.$$"
  $as_root cp -a "$stash/kanban" "$kanban.new.$$"
  swap_dir "$root.new.$$" "$root"
  $as_root mv -f -T --no-copy "$kanban.new.$$" "$kanban"
  $as_root rm -rf "$stash"
  say "イメージの版に戻した（記録と退避を消した）"
  reboot || true
}

status() {
  [ "$#" -eq 0 ] || usage
  fetch
  local head image applied base behind
  head=$(main_head)
  image=$(image_commit)
  applied=$(applied_commit)
  echo "イメージの版: $image"
  if [ -n "$applied" ]; then
    echo "当てた commit: $applied（$(sed -n 's/.*"applied_at": *"\([^"]*\)".*/\1/p' "$record")）"
    base=$applied
  else
    echo "当てた commit: 当てていない"
    base=$(git -C "$repo" rev-parse --verify --quiet "$image^{commit}" || true)
  fi
  warn_image_drift "$image" "$head"
  if [ -n "$base" ] && behind=$(git -C "$repo" rev-list --count "$base..$head" 2>/dev/null); then
    echo "origin/main からの遅れ: $behind（origin/main は $(short "$head")）"
  else
    echo "origin/main からの遅れ: 不明（$image が $repo に無い。origin/main は $(short "$head")）"
  fi
}

case "$cmd" in
  apply) apply "$@" ;;
  status) status "$@" ;;
  revert) revert "$@" ;;
  *) usage ;;
esac
