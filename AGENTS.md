# AGENTS.md（この repo で働く担当の指針）

## 検査を回す

| 決まり | 守らせるもの |
|---|---|
| テストは `go test ./...` と `tests/run --changed`（変えたものに関係する `tests/*.sh` だけを並べて走らせる）。commit の前に通す。手元では関係する検査だけ、全部（`tests/run`）は CI に任せる。 | `.github/workflows/go.yml` と `.github/workflows/tests.yml`。main の ruleset が checks の `go`・`tests` の通過をマージの条件にしている。commit の前に回したかは見ない。 |
| Go のバージョンは `mise.toml`（`go.mod` と揃える）。 | `tests/image.sh`。`go.mod` の go と Dockerfile の golang のバージョンを `mise.toml` と突き合わせる。 |
| `gofmt -l .` が空で、`golangci-lint run ./...` の指摘が 0 件であること（バージョンは `mise.toml`、設定は `.golangci.yml`。CI の go の job も回す）。 | `.github/workflows/go.yml` の gofmt と golangci-lint の step。設定は `.golangci.yml`。 |
| errcheck はテストと出力への書き込み（`fmt.Fprint*`・`io.Writer` の `Write`）を見ない。ほかで戻り値の err を捨てるときは、その行に `//nolint:errcheck // <理由>` を書く。 | `.golangci.yml` の errcheck と nolintlint。linter の名前か理由の無い nolint の注釈で落ちる。 |
| `tests/run` は同時に1本だけ走る（mise の置き場と CPU を取り合う）。2本目は理由を1行出して、前の回が終わるまで待つ。 | `tests/run`。鍵のファイルを flock で取る。`tests/ci.sh` が2本を同時に起動して、2本目が1本目の終わるまで走らないことを確かめる。 |
| `tests/isolation` は15分以上かかるので、hc の端末（`hc run --bg`）で裏に回す。 | 無し。どこで走らせるかは担当の手順で、見るものが無い。 |
| `tests/run --changed` は、重い検査（`andon-dispatcher.sh`・`schema.sh`・`apply-main.sh`・`andon-issue.sh`・`andon-gate.sh`）を、その検査が見る部分を変えたときだけ選ぶ。対応の表は `tests/lib/scope.sh`。重い検査が打つ command や読むファイルを変えたら、表と `docs/image.md` の「手元は関係する検査だけ」を同じ PR で直す。 | `tests/ci.sh`。Go の import の漏れだけを落とす。打つ command・読むファイルと `docs/image.md` の表は見ない。 |
| 約束の JSON は `contract.Write`・`Encode`・`EncodeLine` で書く。schema に照らさずに JSON を作る呼び出し（`json.Marshal` など）を `internal` の下の `_test.go` でない `.go` ファイルに足したら、`internal/contract/writers_test.go` の表に数と理由を足す。`_test.go` のファイルは数えない。 | `go test ./...` の `internal/contract/writers_test.go`。テスト以外のファイルの呼び出しの数を表と突き合わせ、足して表を直さないと落ちる。 |
| 直す前と後の検査の数を比べるときは、直す前を merge-base の木（`git archive` で展開）で数える。書き換え中の worktree や動いている main で数えると、比べる控えにならない。 | 無し。手で数える手順で、見るものが無い。 |
| 検査はセルの本物（kanban・HOME・セルの repo の memory・`CLAUDE_CONFIG_DIR`・`CODEX_HOME`）を読み書きしない。`tests/*.sh` は `tests/lib/andon.sh`・`kanban.sh` を読んで切り離す（`tests/lib/isolate.sh`）。Go の検査は、起動の記録のように置き場を読むコードを通すとき、`HOME`・`ANDON_WORKSPACE` を `t.Setenv` で一時に向け、`CLAUDE_CONFIG_DIR`・`CODEX_HOME` を空にする。 | `tests/lib/isolate.sh` が読んだ検査の環境を切り離す。`tests/isolation` が偽のセルで全部を回し、番兵のファイルが変わるか、番兵のファイルを読む（atime が動く）と落ちる。番兵には中身が空の memory の目録と壊れた JSON の plugin の控えを置く。PR では `.github/workflows/tests.yml` の job isolation が、変更に関係する分を `tests/isolation` で回す。 |
| 検査の土台（`tests/lib/`・`tests/host/` などの検査が読む置き場・HOME や kanban を読む検査）や Go を変えた PR は、`tests/isolation` を通す。 | `.github/workflows/tests.yml` の job isolation。PR の変更から `tests/lib/scope.sh --isolation` が選んだ検査を `tests/isolation` で回し、番兵を読み書きすると落ちる。土台の変更で全部、Go の変更で `tests/smoke.sh` を選ぶことは `tests/ci.sh` が見る。 |
| Go の検査の隔離の漏れは `tests/isolation tests/smoke.sh`（go test と smoke.sh の1本だけ）で数分で再現できる。全部を回すと15分以上かかる。 | 無し。どの検査で再現するかは手順の話で、見るものが無い。 |
| `tests/run --changed` は `package/` に触れると長くかかる。`hc run --bg` で裏に回し、worker skill §3 の 4 の順（返った address で、`reason` が `exit` になるまで `hc wait` を打ち直し、`finished` を確かめてから読んで片付ける。`--rm` は付けない）で待ち、結果をファイルに残して、表の行のうち ok でない行（`grep '^\|' \| grep -v '\| ok \|'`）で失敗を見る。 | 無し。どう回すかは担当の手順で、見るものが無い。 |
| セルには docker が無く、手元の `tests/image.sh` は build と compose を飛ばして ok になる（`tests/lib/run-tests.sh` を変えると選ばれるが、何も見ていない）。image・compose・sshd に触れる変更は、推測でコードを足す前に検査を CI に出して確かめる。 | `.github/workflows/tests.yml` の tests の job。docker のある runner で `tests/image.sh` が回り、CI（`CI=true`）では docker が無いと落ちる。 |
| `review-request` の守りを変えたら、`tests/andon-issue.sh` などで、レビュアーの課題を `todo` のまま結果を置いて作っていないかを見る。動いている（`todo`・`doing`）レビュアーの課題があると頼み直しは断られるので、返した形（`kanban start`・`kanban review`）にし、照合に数えたくないものは Chief of Staff でない端末で足す。 | `tests/andon-issue.sh`。レビュアーの課題が `todo`・`doing` の間は「もう頼んである」と断ることと、返したあとの頼み直しが通ることを見る。 |
| boot の実測は `tests/andon-boot.sh` の `env -i` と一時領域の指定を使う。`HOME` だけを変える手順は使わない。 | 無し。手で実測する手順で、見るものが無い。 |
| 否定の grep（`! grep -q <古い名前>`）には、当たる場合があることを同じ所で確かめる行を添える（添えないと、検査が何も見ていなくても通る）。 | 無し。検査の書き方の決まりで、見るものが無い。 |
| `tests/andon-dispatcher.sh` の偽の `gh` は `pr view 7 --json state,mergeCommit` の回数を settle の検査が数えるので、新しい問い合わせは欄を変えて別の case にする。 | `tests/andon-dispatcher.sh`。同じ欄の問い合わせを足すと、settle の回数の検査が落ちる。 |
| schema の検査器の lint は `maximum` と `\s` を知らずに断る。上限は command で確かめ、空白は明示した文字の組で書く。 | 無し。検査器の lint が知らない書き方は、その場で断られて気づく。 |
| schema の `pattern` は Go の正規表現（google/jsonschema-go）で読まれるので、先読みと `\u` を書かない。 | 無し。検査器の lint が知らない書き方は、その場で断られて気づく。 |
| `tests/package.sh` は実行権限が無いので、`bash tests/package.sh` で走らせる。 | 無し。実行権限の有無は手順の話で、見るものが無い。 |
| `tests/andon-dispatcher.sh` で pending の report を `last` で見ると、担当の review-request が積んだ report を拾う。通知が積まれたかは前後の件数で比べる。 | 無し。検査の書き方の決まりで、見るものが無い。 |
| `tests/package.sh` の仮設の語の検査は `package/skills` の文を「推し」で見るので、「推した」と書かず「推奨は〜だった」と書く。 | `tests/package.sh`。`package/skills` に「推し」を含む文があると落ちる。 |
| `.claude/skills/verify-andon/` は追跡済み。生成物を片付けるときは `.claude` を丸ごと消さず、`git ls-files` と `git status --short` で自分が作ったフォルダだけを消す。 | 無し。片付けの手順で、見るものが無い。 |
| worker skill §2 の「3つの観点」の並びは `tests/package.sh` の検査 20 と 23（yomiyasu の `allow.txt`）が数と語を固定している。観点を足すなら、並びの外の項にする。 | `tests/package.sh` の検査 20 と 23。並びの数と語が変わると落ちる。 |
| `tests/andon-init.sh` の sandbox の `PATH` には `awk` が無い。`bin/bootstrap` はホストの前提が docker と bash だけなので、文字の処理は bash で書き、`awk` などの外の command を足さない。 | 無し。sandbox の `PATH` に無い command は、その場で「見つからない」と落ちて気づく。 |
| 検査の中の偽の鍵と秘密鍵の見出しは、文字列を組み立てて作る（そのまま書くと `tests/public.sh` に当たる）。 | `tests/public.sh`。そのまま書くと当たって落ちる。 |
| 起動の口を足すときは、Go の検査が `hc` の呼び出しの並びを固定しているので、`hc` を余分に呼ぶと `internal/issue`・`chiefengineer`・`chiefofstaff` の検査が落ちる。 | `go test ./...`。`internal/issue` などの検査が `hc` の呼び出しの並びを固定している。 |
| `tests/schema.sh` に新しい schema を足すときは、`schema_of`・`expected`・`$id`・必須の表・`NESTED`・3b の値・`LOOSE`・`EMIT` の8か所を触る。 | `tests/schema.sh`。`schema/` の顔ぶれと `expected`、`$id` のバージョンを突き合わせるので、触り忘れの一部で落ちる。 |
| `\|` で割る表（`LOOSE`・`NESTED`・`EMIT` など、`IFS='\|'` で読むもの）の jq の式には `\|` を書かず、`+=` や `del()` で書く。3b の表は式を最後の `\|` の手前までとして読むので、式の中の `\|` は書いてよい。 | 無し。書いても落ちるとは限らない。 |
| dispatcher の控え（State）の項目を足す・消す・別の記録に移すときは、`cmd/kanban/dispatcherstate.go` の写しと `process/states/` の record か positions も揃える。 | `go test ./...` の `TestDispatcherStateShape`（`cmd/kanban/dispatcherstate_test.go`）と `TestStateFieldsInProcess`（`internal/dispatcher/process_test.go`）。 |
| `tests/*.sh` で `kanban add` する課題には `--repo` を付ける。`kanban init --repo` の値は課題に引き継がれず、課題の repo が空になる。 | 無し。付け忘れても落ちるとは限らず、見るものが無い。 |
| シェルの検査で文字列を比べるときは `[[ "$a" == "$b" ]]` と右辺も引用符で包む。包まないと右辺が glob に読まれて、違う文字列が一致したり、一致すべきものが落ちたりする。 | 無し。書き方の決まりで、見るものが無い。 |
| kanban は課題の番号を正規の形に直す（`S-7` は `S-0007`）。検査で `library/` のパスを組むときは正規の形で書く。 | 無し。書き方の決まりで、見るものが無い。 |
| `tests/andon-dispatcher.sh` は場面が kanban を共有する。場面を足して課題を `doing` のまま残すと、後の場面の数がずれる。足した場面は終わりで `kanban done` する。 | `tests/andon-dispatcher.sh`。残した課題があると、後の場面の数の検査が落ちる。 |
| `set -o pipefail` の下で `cmd \| head -1` を使わず、`sed -n 1p` か `$(…)` で受ける。`cmd` の出力が増えると SIGPIPE で落ちる。 | 無し。出力が少ないうちは通り、見るものが無い。 |
| `tests/` の下のシェルで判定を `A && B` と並べたら、行末に `\|\| die "<何が起きなかったか>"` を付ける。set -e は `&&` の最後でない項と `!` の付いた文では止まらない。if・while の条件と、関数・サブシェルの本体の最後の文には要らない。A が偽なら B を見ずに先へ進む見張り（`[ -n "$want" ] && grep -q …`）は、判定と見分けられないので `if A; then B; fi` と書く。 | `tests/shell-and.sh`。set -e が効く所で、最後の項が判定（`[`・`[[`・`grep`・`cmp`・`jq -e` など）の `A && B` に受け手が無いと、ファイルと行を出して落ちる。`tests/lib/scope.sh` のいつもの分で、CI でもいつも走る。 |
| 検査のシェル（`tests/*.sh`）を直すときは Edit ツールで直し、heredoc で包んだスクリプトで文字列を差し替えない。内側の `EOF` で外側が閉じ、破壊的な行（`rm -f "$HOME/.gitconfig"` など）が本物の HOME で走る。 | `.claude/hooks/test-shell-edit`。`.claude/settings.json` が Claude Code の PreToolUse で呼び、`tests/` の下の検査のシェルへ書く Bash の呼び出し（heredoc で包んだ python・node・perl の書き込み・`sed -i`・`perl -i`・リダイレクト・`tee`）を断る。断る形と通す形は `tests/shell-edit-hook.sh` が見る。字面で見るので、find や一覧を渡す一括置換とファイルに置いたスクリプトの中身は拾わない。Codex には効かない。 |
| 否定の検査で注釈の行を外すときは、行頭の空白と `#` で外す（`^[^#]*` は `"${#arr[@]}"` の `#` に当たり、行を全部落とす）。 | 無し。書き方の決まりで、見るものが無い。 |
| 対話シェルの grep は ugrep の関数で、ファイルが1つでもファイル名を出す。`bash tests/*.sh` の中は GNU grep で、ファイルが1つだと出さない。出力の形を確かめるときは `/usr/bin/grep` か `bash -c` で打つ。 | 無し。確かめ方の手順で、見るものが無い。 |
| 表の「守らせるもの」の列に repo に無いファイル（`gate.local.json` など）を `…` で書くと `tests/agents-md.sh` が落ちる。repo の外の置き場は「決まり」の列に書く。 | `tests/agents-md.sh`。守らせるものの `…` に書いたパスの語が repo に無いと落ちる。 |
| `tests/smoke.sh` の否定の grep は Go の検査の関数名にも当たる。消す識別子を含む名前を検査に付けない。 | 無し。名前の付け方の決まりで、見るものが無い。 |
| gate の差分の見本を repo に置くときは、元の PR の削除行に古い語が残って `tests/words.sh` に当たる。当たった所の前後で行を切って置く。 | `tests/words.sh`。CI でいつも走る。 |
| andon の schema の検査器は `uniqueItems` を知らず、schema に書くと command が落ちる。重なりは Go の Parse で見る。 | 無し。検査器の lint が知らない書き方は、その場で断られて気づく。 |
| 直す前の木で新しい検査が落ちるかを見るときは、ローカルの main でなく自分の HEAD から一時の worktree を切る。 | 無し。確かめ方の手順で、見るものが無い。 |
| `tests/schema.sh` を単独で `andon issue record` に撮らない。emitters を全部走らせ直して10分を超える。schema の証拠は `tests/run --changed` の中で撮る。 | 無し。撮り方の手順で、見るものが無い。 |
| `andon process render` は、その worktree で `mise exec -- go build ./cmd/andon` で作った andon で打つ。セルの andon は古いバージョンのことがある。 | `tests/process.sh`。`docs/process/` が `process/` から作り直しても変わらないことを見る。どの andon で打ったかは見ない。 |
| 手元に docker が無いと `tests/image.sh` は飛ばされる。boot の `gh` の呼び出しを変えたら、偽の `gh` の部分を `env -i` と一時の HOME で `andon boot` に打たせて確かめる。 | 無し。手で確かめる手順で、見るものが無い。 |
| セルに入れたバイナリで `tests/andon-dispatcher.sh` を回すときは、`ANDON_BIN` に `readlink -f` で引いた実体のパス（`/opt/andon/bin/andon` の実体）を渡す。symlink のままだと「シェルに替わった dispatcher を片付けて起動し直さない」の検査が誤って落ちる。 | 無し。どのバイナリで回すかは手順の話で、見るものが無い。 |
| go が mise の shim のとき、`tests/isolation` は「番兵に書いた」で誤って落ちる。`PATH="$(dirname "$(mise which go)"):$PATH" tests/isolation …` と実体を先頭に置いて回す。 | 無し。job isolation の runner に mise は無く、手元でどの go で回したかを見るものが無い。 |
| `tests/andon-dispatcher.sh` の merge・settle の見張りの時刻は `freeze` が止める。直に打つ口を足すときは先に `freeze` を呼び、聞き直させたい課題だけ `mpoll`・`watch_due` で due にする。 | `tests/andon-dispatcher.sh`。`freeze` を呼ばずに打つと時刻が進み、後の場面の問い合わせの数の検査が落ちる。 |
| `internal/chiefofstaff` の `TestChiefOfStaffForegroundUnknown` と `internal/dispatcher` の `TestRewakeSkipsWhenNoLongerStopped` は、`tests/run` と `go test ./...` を並べるなど負荷の下で揺れる。落ちたら単独で回し直す。 | 無し。負荷で揺れるので見るものが無い。 |
| この表の「守らせるもの」の列は、repo に在るパスを名指す。無いものを名指さない。 | 無し。書き方の決まり。 |

## 約束と文書

| 決まり | 守らせるもの |
|---|---|
| 約束の正は `schema/`。schema を変えたら `docs/contract.md` と tests も直す。 | `tests/schema.sh`。`docs/contract.md` の例を schema と突き合わせる。 |
| andon の葉を足す・変えるときは `internal/cli/leaves.go` の表に載せ、`andon help --all --markdown >docs/commands.md` と `andon help --all --bash >image/completions/andon` で作り直す。 | `tests/commands.sh`。作り直したものと比べ、`package/` の skill・フック・テンプレートが名指すコマンドを表と照らす。 |
| `go:embed` で repo のファイルを足したら、`image/Dockerfile` の `COPY` と `.dockerignore` も直す。 | `tests/image.sh`。image の build で、`go build` が埋め込むファイルを見つけられずに落ちる。 |
| `package/` の変更のうち、検証の skill（`.claude/skills/verify-andon/`）の機能の地図の「変えたら動かすパス」に当たるものは、マージしてセルへ当てたあと（`image/apply-main.sh apply`）、検証の課題が当たる機能をセルの本物で動かす。検証の課題は Chief of Staff が `package/skills/chief-of-staff/playbooks/verify.md` の手順で切る。地図に無いパス（`package/lib/`・`package/host/` など）の変更では、検証の課題は切られない。`internal/`・`cmd/`・`package/` にファイルや package を足したら、同じ PR で地図を直す。「変えたら動かすパス」の下なら、それを動かす細かい機能の「見るファイル:」に足す。どの機能にも当たらないなら、機能のパスに足すか、地図の目次（`features/README.md`）の「地図が覆わないもの」に理由と書く。 | `tests/verify-andon.sh`。機能の地図の形と、地図の「変えたら動かすパス」が repo に在ること、`.claude/skills/verify-andon/scripts/features.sh` が差分から機能を引くことを見る。「変えたら動かすパス」の下のテストでないファイルが、どれかの細かい機能の見るファイルに入ることと、`internal/`・`cmd/`・`package/` の下のテストでないファイルが、地図か「地図が覆わないもの」に入ることも見る。細かい機能が名指したファイルを本当に動かすかは見ない。 |
| `package/skills` の文を `playbooks/` へ移したら、`tests/words.sh` の禁止語に当たらないか回す。 | `tests/words.sh`。CI でいつも走る。 |
| 報告や依頼書の文を `docs/`・注釈に写したら、commit の前に `bash tests/words.sh` を回す（報告・依頼書は語の検査を通らず、禁止語が残っている）。 | `tests/words.sh`。CI でいつも走る。 |
| README を直すときは、同じ PR で `README.md`（英語）と `README.ja.md`（日本語）の両方を直す。文の正は `README.ja.md`（ADR-0046 決めたこと 3）。 | `tests/readme.sh`。2 つの見出しの並び・相対リンクの先・command・図のノードと矢印がずれると落ちる。文の意味のずれは見ない。 |
| 設計の正は `docs/design.md` と `docs/adr/`。 | 無し。どれを正とするかの決まりで、見るものが無い。 |
| Chief Engineer の skill の骨格の正は `docs/design.md` §1.8。`package/skills/chief-engineer/SKILL.md` の手順を変える PR は、同じ PR で §1.8 も直す。 | 無し。`tests/package.sh` は skill の有無を見るだけで、SKILL.md と §1.8 の食い違いは見ない。 |
| 決定の記録は `docs/decisions/` に1件1ファイルで足す（書き方は `docs/decisions.md` の冒頭。`docs/decisions.md` の表には行を足さない）。 | `tests/smoke.sh`。`docs/decisions/` のファイルの名前と行の形、`docs/decisions.md` の凍結した表に行が足されていないことを見る。 |
| ADR の番号・状態行・覆し方は `docs/design.md` §1.8 の「置き場」。 | `tests/adr-index.sh`。ADR の状態行と索引、決定の記録を突き合わせ、`docs/adr/README.md` の一覧の行が在る ADR を指すことを見る。 |
| ADR は `tests/adr-new <英字の短い名前> "<タイトル>"` で始める。番号を手で決めない。同じ番号の ADR が main と重なったら、あとからマージする側が取り直す。 | `tests/adr-index.sh`。`docs/adr/` に同じ番号の ADR が 2 本あると落ちる。 |
| ADR の PR は `docs/adr/README.md` の一覧に触らない。一覧は、バージョンを切る PR が `bash tests/adr-index.sh --write-list` で置き換える。 | `tests/adr-index.sh`。一覧の見出しの下のバージョンを `CHANGELOG.md` の一番上のバージョンと突き合わせ、違えば落ちる。ADR ごとの行は求めない。 |
| ADR を足したら、決定の記録にも ADR 1本につき1件を `docs/decisions/YYYY-MM-DD-adr-NNNN-<題>.md` で足す。 | `tests/adr-index.sh`。ADR ごとに `docs/decisions/` の対か `docs/decisions.md` の凍結した表（§14）の行が無ければ落ちる。 |
| 迷ったら推奨案で進めて報告に書く。 | 無し。判断の決まりで、見るものが無い。 |
| 1ファイル1読み手。人用の文書（`README.md`・`docs/`）に機械が読む行を埋めない。AI 用の文書（この `AGENTS.md`・`REVIEW.md`・`package/skills/`）に人向けの経緯を書かない。 | 無し。読み手は文の中身で決まり、機械では見分けない。 |
| この repo は public。秘密・個人の環境に固有の値（セル名・ホスト名・ポート番号・メールアドレス・ユーザー名・ホームのパス）・案件に固有の名前を入れない。例は架空の名前（`example.com`・`<owner>`・`sample-site`）で。禁止リストは repo の外に置く（書き方は `tests/README.md`。リストの語を repo に書かない）。 | `tests/public.sh`。禁止リストと秘密の形に当たる行で落ちる。 |
| この表の決まりは「決まり｜守らせるもの」の対で書く。守らせるものは、検査・command・hook・repo の設定の名前か、「無し。」と理由。 | `tests/agents-md.sh`。表の外の決まり・対になっていない行・名指したファイルが無い行で落ちる。CI でいつも走る。 |
| settle（オーナーのマージの片付け）の約束は、コードの注釈・`docs/dispatcher.md`・`docs/contract.md`・`process/states/human-merge.yaml`・`docs/decisions/` に写されている。settle を変えたら全部を揃え、YAML を変えたら `andon process render` で `docs/process/` を作り直す。 | `tests/process.sh`。`docs/process/` が `process/` から作り直しても変わらないことを見る。ほかの写しの食い違いは見ない。 |
| ADR の決定の索引の「今の正」は、実物の `ls`・`grep` で在るパスを確かめてから書く。 | 無し。書き方の決まり。 |

## PR とマージ

| 決まり | 守らせるもの |
|---|---|
| `.github/workflows/**` と `tests/public*.sh` を弱める変更はオーナーが見る。 | `package/gate.json` の `tier_c_paths`。gate がレベル C にし、委譲の `merge` が C でなければオーナーのマージに回す。 |
| 課題の一生の動き（finish・resume・gate の返し方など、`model/IssueLife.tla` が写しているコード）を変える PR は、同じ PR で `model/` のモデルも直す。回すのは `model/check`（ADR-0025）。 | `model/check` を `.github/workflows/model.yml` が回し、モデルと `process/states/issue.yaml` の状態を対応表で照らす。コードとモデルのずれは見ない。 |
| 規則（`model/IssueRules.*`）・控え（`model/expected.txt`）・対応表（`model/issue-map.txt`）を弱める・消す変更は、既定の条件表でレベル C になり、オーナーが見る。Andon を開発するセルでは、v0.0.x の間、オーナーがセルの条件表の上書きでレベル B に下げる（ADR-0041 決めたこと 10・11）。 | `package/gate.json` の `tier_c_paths`。gate がレベル C にし、委譲の `merge` が C でなければオーナーのマージに回す。 |
| 課題の一生に触れる設計を Chief Engineer がモデルで調べるときは、`model/` のモデルと規則を自分の課題の資料置き場に写して案を書き足し、statechart と `model/check --draft <フォルダ>` の両方で調べる（statechart は「行ける」の形の規則を調べない。口の中身は `model/README.md` の「下書きを調べる」）。写しの控え・対応表・壊した見本は直さず、足す規則と変わる見込みの控えを文書に文で書く（直すのは実装の PR の担当）。 | 無し。Chief Engineer の検討の手順で、どの道具で調べたかを見るものが無い。 |
| main へ直接 push しない。 | GitHub の ruleset `main`。PR を通さない push を断る。 |
| マージはオーナーがする。 | `package/settings.json` が担当の `gh pr merge` を deny する。 |
