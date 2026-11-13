# tests/*.sh が source する: 台帳の CLI（kanban）を用意し、$kanban に入れて PATH の先頭に置く。
#   KANBAN_BIN があればそれを使う（CI が先に焼いたもの）。無ければ go build で一時ディレクトリに焼く。
#   go が無ければ（mise があれば mise exec で）、どちらも無ければ skip して終わる（CI では落とす）。
# 呼ぶ側は $tmp（後片付けする一時ディレクトリ）を先に作っておく。
. "$(dirname "${BASH_SOURCE[0]}")/isolate.sh"
kanban_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
mkdir -p "$tmp/kanban-bin"
if [ -n "${KANBAN_BIN:-}" ]; then
  ln -sf "$KANBAN_BIN" "$tmp/kanban-bin/kanban"
elif command -v go >/dev/null; then
  (cd "$kanban_root" && go build -o "$tmp/kanban-bin/kanban" ./cmd/kanban) || { echo "FAIL go build ./cmd/kanban" >&2; exit 1; }
elif command -v mise >/dev/null; then
  (cd "$kanban_root" && mise exec -- go build -o "$tmp/kanban-bin/kanban" ./cmd/kanban) || { echo "FAIL go build ./cmd/kanban" >&2; exit 1; }
else
  if [ "${CI:-false}" = true ]; then echo 'FAIL CI requires go (or KANBAN_BIN)' >&2; exit 1; fi
  echo "skip: go が無い（KANBAN_BIN も無い）"; exit 0
fi
kanban=$tmp/kanban-bin/kanban
[ -x "$kanban" ] || { echo "FAIL kanban が無い: $kanban" >&2; exit 1; }
export PATH="$tmp/kanban-bin:$PATH"
