// Package digest はダイジェスト（ADR-0044）: 工場で起きたことを、オーナーが後から短い時間で理解できる 1 枚にまとめる。
//
//	andon digest collect <id>   期間の記録を library/digest/input.json に集める（読むだけ。書くのはその1つ）
//	andon digest build <id> -   担当が書いた要旨・テーマ・気になること（標準入力の JSON）を照らし、digest.json・digest.html・message.txt を書く
//	andon digest send <id>      digest.html を files/ と project.json の送り先（Slack）へ出し、結果を sent.json に書く
//	andon digest check          project.json の送り先へ、確かめの小さなファイルを send と同じ口で送る
//	andon digest auth slack     Slack の bot のトークンをセルの中で聞いて置く
//
// セルの外へ送るのは send と check だけで、送る先は project.json の digest の送り先だけ（決めたこと 12）。
// 数と一覧と図は、この package が入力から描く（決めたこと 8）。担当が書くのは文だけ。HTML は依存の無い単一のファイルで、
// スクリプトを持たない（決めたこと 7）。入力と担当の JSON の文字列は、組み立てる前に秘密の形を伏せる（決めたこと 17）。
// 形は docs/contract.md §26〜§28。
package digest

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/dispatcher"
	"andon/internal/ledger"
	"andon/internal/measure"
	"andon/internal/signal"
)

// Usage は andon digest の使い方。
const Usage = "usage: andon digest collect <id>          期間の記録を library/digest/input.json に集める（読むだけ。docs/contract.md §26）\n" +
	"       andon digest build <id> <file|->     担当が書いた JSON（§27）を照らし、digest.json・digest.html・message.txt を書く\n" +
	"       andon digest send <id>               digest.html を files/ と送り先へ出し、URL を出して sent.json（§28）に書く\n" +
	"       andon digest check                   送り先へ確かめの小さなファイルを送る\n" +
	"       andon digest auth slack              Slack の bot のトークン（scope は files:write だけ）をセルの中で聞いて置く\n" +
	"<id> はダイジェストの課題。期間は、同じルーチンの前に送ったダイジェストの終わりから今まで（無ければ 24 時間前から、7 日より前へは遡らない）。\n" +
	"collect は kanban と dispatcher の記録と GitHub のマージされた PR の一覧と、課題の repo の最新の main の文書（代わりに決めたこと）を読み、書くのは input.json だけ。\n" +
	"build は入力に無い課題を指した・要旨が 2〜5 文でない・テーマが 9 つ以上・知らない図の型などを理由1行ずつで断り、何も書かない。\n" +
	"送り先は files/ と、オーナーが project.json の digest に書いたチャンネルだけ（§11）。無い・信じられなければ files/ だけに出す。\n" +
	"send は送った先に打ち直しても送らない。送りそこねたら終了コード 1 で、打ち直すと送り直す。\n" +
	"auth は標準入力の1行を ~/.config/andon/digest/slack.token（mode 600）に置く。トークンを引数に書かない。\n" +
	"projects/<slug>/ か担当の worktree で打つ（KANBAN_ROOT でも向けられる）。\n"

// 置き場（課題の library/digest/）。
const (
	dirName     = "digest"
	inputFile   = "input.json"
	digestFile  = "digest.json"
	htmlFile    = "digest.html"
	messageFile = "message.txt"
	sentFile    = "sent.json"
)

const timeLayout = "2006-01-02T15:04:05Z"

// Env は command が外から読むもの。テストで差し替える。
type Env struct {
	Root      string    // プロジェクトの根（kanban/ の親）
	Workspace string    // セルの repo（files/ の親）
	Hostname  string    // files の URL のホスト名に使うセルの名前
	Now       time.Time // 集める時刻・送る時刻
	// GH は repo（<owner>/<repo>）のマージされた PR の一覧（measure.GH と同じ形）。nil なら GitHub を読まない。
	GH func(repo string) ([]byte, error)
	// Delivered・Signals・State は dispatcher の記録（届けた通知・外の信号・控え）。空なら読まない。
	Delivered, Signals, State string
	// ClaudeDir・CodexSessions はトークンを数えるセッションの記録（andon measure と同じ）。空なら読まない。
	ClaudeDir, CodexSessions string
	// SlackAPI は Slack の Web API の根、Tokens はトークンの置き場。空なら Slack へ送らない（検査が本物へ届かない）。
	SlackAPI, Tokens string
	// Stderr は誤りにしない知らせの出し先（送り先を読めない、など）。nil なら出さない。
	Stderr io.Writer
}

func (e Env) warn(format string, a ...any) {
	if e.Stderr != nil {
		fmt.Fprintf(e.Stderr, "andon digest: "+format+"\n", a...)
	}
}

// Main は andon digest。
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run(args, stdin, stdout, stderr, defaultEnv)
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, newEnv func() (Env, error)) int {
	usage := func() int { fmt.Fprint(stderr, Usage); return 2 }
	fail := func(err error) int { fmt.Fprintf(stderr, "andon digest: %v\n", err); return 1 }
	switch {
	case len(args) == 2 && args[0] == "auth":
		// 引数は誤りに写さない（トークンを貼り間違えても出力に残さない）
		if args[1] != kindSlack {
			fmt.Fprintln(stderr, "andon digest auth: 知らない送り先（slack）。トークンは引数に書かず、標準入力に渡す")
			return 2
		}
		const prompt = "Slack の bot のトークン（scope は files:write だけで作る）"
		if err := signal.PlaceToken(TokenDir(), kindSlack, prompt, stdin, stderr); err != nil {
			fmt.Fprintf(stderr, "andon digest auth: %v\n", err)
			return 2
		}
		return 0
	case len(args) == 1 && args[0] == "check":
		env, err := newEnv()
		if err != nil {
			return fail(err)
		}
		if err := CheckDestinations(env, stdout); err != nil {
			return fail(err)
		}
		return 0
	case len(args) < 2:
		return usage()
	}
	cmd, id := args[0], ledger.CanonID(args[1])
	switch {
	case cmd == "collect" && len(args) == 2, cmd == "send" && len(args) == 2, cmd == "build" && len(args) == 3:
	default:
		return usage()
	}
	if !idRe.MatchString(id) {
		fmt.Fprintln(stderr, "andon digest: id が不正")
		return 1
	}
	env, err := newEnv()
	if err != nil {
		return fail(err)
	}
	env.Stderr = stderr
	switch cmd {
	case "collect":
		in, err := Collect(env, id)
		if err != nil {
			fmt.Fprintf(stderr, "andon digest: %v\n", err)
			return 1
		}
		path, err := writeInput(env.Root, id, in)
		if err != nil {
			fmt.Fprintf(stderr, "andon digest: %v\n", err)
			return 1
		}
		for _, s := range in.Skipped {
			fmt.Fprintf(stderr, "andon digest: 読めなかった %s: %s\n", s.Source, s.Reason)
		}
		fmt.Fprintln(stdout, path)
		return 0
	case "build":
		b, err := readArg(args[2], stdin)
		if err != nil {
			fmt.Fprintf(stderr, "andon digest: %v\n", err)
			return 1
		}
		paths, errs := Build(env, id, b)
		if len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintf(stderr, "andon digest: %s\n", e)
			}
			return 1
		}
		for _, p := range paths {
			fmt.Fprintln(stdout, p)
		}
		return 0
	default:
		url, err := Send(env, id)
		if url != "" {
			fmt.Fprintln(stdout, url)
		}
		if err != nil {
			return fail(err)
		}
		return 0
	}
}

func defaultEnv() (Env, error) {
	root := os.Getenv("KANBAN_ROOT")
	if root == "" {
		l, err := ledger.Open()
		if err != nil {
			return Env{}, err
		}
		root = l.Root
	}
	host, err := os.Hostname()
	if err != nil {
		return Env{}, fmt.Errorf("ホスト名を読めない: %v", err)
	}
	slug := filepath.Base(root)
	return Env{Root: root, Workspace: delegate.Workspace(), Hostname: host, Now: time.Now().UTC(), GH: measure.GH,
		Delivered: dispatcher.DeliveredPath(slug), Signals: signal.Path(slug), State: dispatcher.StatePath(slug),
		ClaudeDir: measure.ClaudeDir(), CodexSessions: measure.CodexSessions(),
		SlackAPI: slackAPI, Tokens: TokenDir()}, nil
}

func readArg(arg string, stdin io.Reader) ([]byte, error) {
	if arg == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(arg)
}

// dir は課題 id の library/digest/。
func dir(root, id string) string {
	return filepath.Join(ledger.At(root).Library(id), dirName)
}

func writeInput(root, id string, in *Input) (string, error) {
	d := dir(root, id)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(d, inputFile)
	if err := contract.Write(path, contract.SchemaDigestInput, in); err != nil {
		return "", fmt.Errorf("input.json が schema/digest-input.schema.json に合わない（書かない）: %v", err)
	}
	return path, nil
}

// readInput は input.json を読み、schema とオーナー待ちの schema に照らす。
func readInput(root, id string) (*Input, error) {
	path := filepath.Join(dir(root, id), inputFile)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s が無い（先に andon digest collect %s）", path, id)
	}
	if err != nil {
		return nil, err
	}
	if err := contract.SchemaDigestInput.Check(b); err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	var in Input
	if err := contract.Decode(b, &in); err != nil {
		return nil, fmt.Errorf("%s が読めない: %v", path, err)
	}
	if _, err := contract.Encode(contract.SchemaWaiting, in.Waiting); err != nil {
		return nil, fmt.Errorf("%s の waiting: %v", path, err)
	}
	if in.Issue != id {
		return nil, fmt.Errorf("%s は課題 %s のもの（%s でない）", path, in.Issue, id)
	}
	return &in, nil
}
