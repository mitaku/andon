# common.sh: journeys/*.sh が（sandbox.sh・cell.sh から）source する。drive.sh が渡す値と、証拠を出す関数。
# 読んだあとに使えるもの: $tmp（一時の置き場。drive.sh が片付ける）・$run（跡の印）・ok・fail・$fails・step <説明> <command…>
tmp=${VERIFY_TMP:?drive.sh から呼ぶ}
run=${VERIFY_RUN:?drive.sh から呼ぶ}
fails=0
ok() { echo "ok   $*"; }
fail() { echo "FAIL $*"; fails=$((fails + 1)); }
# step <説明> <command…>: 打った command と終了コードと出力を証拠に出す。出力は $tmp/step.out にも残る。終了コードを返す
step() {
  local what=$1 rc=0; shift
  echo "-- $what"
  echo "\$ $*"
  "$@" >"$tmp/step.out" 2>&1 || rc=$?
  sed 's/^/  | /' "$tmp/step.out"
  echo "  終了コード $rc"
  return "$rc"
}
