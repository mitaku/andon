package waiting

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"andon/internal/ledger"

	"go.yaml.in/yaml/v3"
)

// AnswerUsage は andon waiting answer の使い方。
const AnswerUsage = "usage: andon waiting answer <id>#<n> --option <番号> [--project <slug>]\n" +
	"オーナーのペインから、開いている判断の問いに、問いの JSON の選択肢の1つ（番号は 1 から）で答える。\n" +
	"書くのは kanban answer の1つだけで、答えの文は選んだ選択肢の label。\n" +
	"断るもの: herdr の端末の中（HC_TERMINAL_ID がある）で打たれた・問いが開いていない・問いの JSON が無い・gate の問い・\n" +
	"判断の種類（business・users・promise・line）でない・種類が irreversible・番号が選択肢の外。断ったら何も書かずに終了コード 1。\n" +
	"docs/adr/0042-owner-waits-do-not-stop-the-line.md 決めたこと 17・18。docs/contract.md §25。\n"

// answerKinds はペインから答えられる問いの種類（灯りの種類から irreversible を除く。決めたこと 18）。
var answerKinds = []string{"business", "users", "promise", "line"}

// Answer は andon waiting answer。
func Answer(args []string, stdout, stderr io.Writer) int {
	ref, option, slug := "", "", ""
	for len(args) > 0 {
		switch {
		case args[0] == "--option" && len(args) > 1:
			option, args = args[1], args[2:]
		case args[0] == "--project" && len(args) > 1:
			slug, args = args[1], args[2:]
		case ref == "" && !strings.HasPrefix(args[0], "-"):
			ref, args = args[0], args[1:]
		default:
			fmt.Fprint(stderr, AnswerUsage)
			return 2
		}
	}
	if ref == "" || option == "" {
		fmt.Fprint(stderr, AnswerUsage)
		return 2
	}
	// 端末の中のエージェント（担当・Chief of Staff）は書けない。App の exec は端末の外で動く（決めたこと 18）
	if id := os.Getenv("HC_TERMINAL_ID"); id != "" {
		fmt.Fprintf(stderr, "andon waiting answer: herdr の端末の中（%s）では答えを書かない。ペインの答えは端末の外から打つ（エージェントは使わない）\n", id)
		return 1
	}
	root, err := projectRoot(slug)
	if err != nil {
		fmt.Fprintf(stderr, "andon waiting answer: %v\n", err)
		return 1
	}
	id, n, label, err := choose(root, ref, option)
	if err != nil {
		fmt.Fprintf(stderr, "andon waiting answer: %s: %v\n", ref, err)
		return 1
	}
	// kanban answer が開いている問いだけを通す（読んでから書くまでに答えが入っても、2つ目は断られる。モデルの R10）
	qref := fmt.Sprintf("%s#%d", id, n)
	if _, err := ledger.At(root).Run("", "answer", qref, label); err != nil {
		fmt.Fprintf(stderr, "andon waiting answer: %s に書けない: %v\n", qref, err)
		return 1
	}
	fmt.Fprintf(stdout, "%s に「%s」と答えた\n", qref, label)
	return 0
}

// choose は問いを確かめ、書く問い（id・n）と答えの文（選んだ選択肢の label）を返す。断るときは理由。
func choose(root, ref, option string) (string, int, string, error) {
	id, nstr, ok := strings.Cut(ref, "#")
	n, err := strconv.Atoi(nstr)
	if !ok || id == "" || err != nil || n < 1 {
		return "", 0, "", errors.New("問いは <id>#<n> の形で書く")
	}
	id = ledger.CanonID(id)
	dir := filepath.Join(root, "kanban", "issues", id)
	b, err := os.ReadFile(filepath.Join(dir, "questions.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		return "", 0, "", errors.New("課題に問いが無い")
	}
	var qs []question
	if err != nil || yaml.Unmarshal(b, &qs) != nil {
		return "", 0, "", errors.New("questions.yaml が読めない")
	}
	i := slices.IndexFunc(qs, func(q question) bool { return q.N == n })
	if i < 0 {
		return "", 0, "", errors.New("その番号の問いが無い")
	}
	if s := qs[i].Status; s != "open" {
		return "", 0, "", fmt.Errorf("問いが開いていない（%s）。答え済みの問いには書かない", s)
	}
	ask := readAsk(filepath.Join(dir, "library"), n)
	if ask == nil {
		return "", 0, "", errors.New("問いの JSON（library/questions/<n>.json）が無いか読めない。選択肢の無い問いは Chief of Staff との会話で答える")
	}
	if ask.From == "gate" {
		return "", 0, "", errors.New("gate の問い（マージの依頼）はペインから答えない。PR を開いて GitHub でマージする")
	}
	if ask.Kind == "irreversible" {
		return "", 0, "", errors.New("戻せない決断（irreversible）はペインから答えない。Chief of Staff との会話で答える")
	}
	if !slices.Contains(answerKinds, ask.Kind) {
		return "", 0, "", fmt.Errorf("判断の問いでない（%s）", ask.Kind)
	}
	k, err := strconv.Atoi(option)
	if err != nil || k < 1 || k > len(ask.Options) {
		return "", 0, "", fmt.Errorf("選択肢の番号は 1 から %d（%q）", len(ask.Options), option)
	}
	return id, n, ask.Options[k-1].Label, nil
}
