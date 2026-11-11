// Package cli は andon の sub-command を振り分ける。
//
//	andon help              一覧
//	andon <sub> --help      その sub-command の使い方
//	andon <sub> [引数…]     中身に渡す
//
// 知らない sub-command は一覧を stderr に出して終了コード 2。
package cli

import (
	"fmt"
	"io"

	"andon/internal/boot"
	"andon/internal/chiefengineer"
	"andon/internal/chiefofstaff"
	"andon/internal/create"
	"andon/internal/delegate"
	"andon/internal/dispatcher"
	"andon/internal/gate"
	"andon/internal/issue"
	"andon/internal/measure"
	"andon/internal/open"
	"andon/internal/plantinit"
	"andon/internal/process"
	"andon/internal/publish"
	"andon/internal/signal"
	"andon/internal/triage"
)

type command struct {
	name  string
	about string
	usage string
	run   func(args []string, stdin io.Reader, stdout, stderr io.Writer) int
}

var commands = []command{
	{
		name:  "issue",
		about: "担当を起こす（start）・担当が台帳に返す（show・report・review-request・ready・ask）・片付ける（finish）",
		usage: issue.Usage,
		run:   issue.Main,
	},
	{
		name:  "chief-of-staff",
		about: "プロジェクトの Chief of Staff を起こす（動いていれば、そのまま使う）",
		usage: chiefofstaff.Usage,
		run:   chiefofstaff.Main,
	},
	{
		name:  "create",
		about: "プロジェクトを立てる（projects/<slug>/ に台帳・project.json の下書き・AGENTS.md。足りないものは聞く）",
		usage: create.Usage,
		run:   create.Main,
	},
	{
		name:  "open",
		about: "プロジェクトを選んで Chief of Staff を開く（andon open <slug>。slug 無しは fzf で選ぶ）",
		usage: open.Usage,
		run:   open.Main,
	},
	{
		name:  "chief-engineer",
		about: "人と仕様・設計を対話する Chief Engineer を、台帳の課題として起こす",
		usage: chiefengineer.Usage,
		run:   chiefengineer.Main,
	},
	{
		name:  "dispatcher",
		about: "台帳を回り、インナーループ（レビュアーを起こす・ゲート・merge: auto のマージと片付け）を回す。通知を届ける口はまだ無い",
		usage: dispatcher.Usage,
		run:   dispatcher.Main,
	},
	{
		name:  "news",
		about: "Chief of Staff が届いた <news> に人へ言うことが無いと示す（ack。ラベル news.ack=<turn>）",
		usage: dispatcher.NewsUsage,
		run:   dispatcher.NewsMain,
	},
	{
		name:  "publish",
		about: "生成したファイルを files/ へ公開し、URL を返す",
		usage: "usage: andon publish <file> [slug]\n" +
			"/workspace/files/ に日付つきでコピーし、URL を返す。同じ日に同じ slug で打つと上書き。\n",
		run: publish.Main,
	},
	{
		name:  "triage",
		about: "台帳の溜まったものを集める（collect）・仕分けの決定を台帳に当てる（apply）",
		usage: triage.Usage +
			"projects/<slug>/ で打つ。collect は読むだけ（担当が打つ）。apply は Chief of Staff が打つ。\n",
		run: triage.Main,
	},
	{
		name:  "signal",
		about: "外の出所（GitHub の main の CI・issue）の信号を読んで記録する（collect）・出所のトークンを置く（auth）",
		usage: signal.Usage,
		run:   signal.Main,
	},
	{
		name:  "measure",
		about: "台帳と記録からラインの流れ（待ち時間・流量・WIP）・効率・対抗の指標を数える（読むだけ）",
		usage: measure.Usage,
		run:   measure.Main,
	},
	{
		name:  "process",
		about: "流れと状態の定義（process/ の YAML）から人が読む docs/process/<名>.md を作る（render）",
		usage: process.Usage,
		run:   process.Main,
	},
	{
		name:  "boot",
		about: "パッケージからセルへ配布物を生成し、portless・sshd・dispatcher を起こす（毎起動）",
		usage: boot.Usage,
		run:   boot.Main,
	},
	{
		name:  "init",
		about: "セルの repo の雛形（<slug>-cell）を作る",
		usage: plantinit.Usage,
		run:   plantinit.Main,
	},
	{
		name:  "gate",
		about: "台帳の課題の PR を予約キー・findings.json・条件表・委譲で照合し、記録を書く（マージはしない）",
		usage: gate.Usage,
		run:   gate.Main,
	},
	{
		name:  "delegate",
		about: "委譲（projects/<slug>/project.json の delegation）を読む（show）・人が書く（grant・narrow・revoke）",
		usage: delegate.Usage,
		run:   delegate.Main,
	},
}

func list(w io.Writer) {
	fmt.Fprint(w, "usage: andon <sub-command> [引数…]\n\n")
	width := len("help")
	for _, c := range commands {
		width = max(width, len(c.name))
	}
	for _, c := range commands {
		fmt.Fprintf(w, "  %-*s  %s\n", width, c.name, c.about)
	}
	fmt.Fprintf(w, "  %-*s  %s\n", width, "help", "この一覧")
	fmt.Fprint(w, "\nandon <sub-command> --help で、その sub-command の使い方を出す。台帳は台帳の CLI で書く。\n")
}

// Run は andon を args で走らせ、終了コードを返す。
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	sub := "help"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "help", "-h", "--help":
		list(stdout)
		return 0
	}
	for _, c := range commands {
		if c.name != sub {
			continue
		}
		rest := args[1:]
		if len(rest) > 0 && (rest[0] == "-h" || rest[0] == "--help") {
			fmt.Fprint(stdout, c.usage)
			return 0
		}
		return c.run(rest, stdin, stdout, stderr)
	}
	fmt.Fprintf(stderr, "andon: 知らない sub-command: %s\n", sub)
	list(stderr)
	return 2
}
