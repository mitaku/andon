# tests/*.sh が source する: andon のバイナリを用意し、$andon に入れる。
#   ANDON_BIN があればそれを使う（CI が先に焼いたもの）。無ければ go build で一時ディレクトリに焼く。
#   go が無ければ（mise があれば mise exec で）、どちらも無ければ skip して終わる。
# 呼ぶ側は $tmp（後片付けする一時ディレクトリ）を先に作っておく。
andon_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
if [ -n "${ANDON_BIN:-}" ]; then
  andon=$ANDON_BIN
else
  andon=$tmp/andon-bin/andon
  if command -v go >/dev/null; then
    (cd "$andon_root" && go build -o "$andon" ./cmd/andon) || { echo "FAIL go build" >&2; exit 1; }
  elif command -v mise >/dev/null; then
    (cd "$andon_root" && mise exec -- go build -o "$andon" ./cmd/andon) || { echo "FAIL go build" >&2; exit 1; }
  else
    echo "skip: go が無い（ANDON_BIN も無い）"; exit 0
  fi
fi
[ -x "$andon" ] || { echo "FAIL andon が無い: $andon" >&2; exit 1; }
