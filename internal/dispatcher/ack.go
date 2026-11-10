package dispatcher

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// NewsUsage は andon news の使い方。
const NewsUsage = "usage: andon news ack\n" +
	"Chief of Staff が、届いた <news> に人へ言うことが無いときに打つ（ADR-0010 決めたこと 4）。\n" +
	"自分の端末（HC_TERMINAL_ID）にラベル news.ack=<turn>（いまの hc の turn）を書く。dispatcher（drain と見回り）は、\n" +
	"<news> を渡したときに控えた turn とこの値が同じなら、そのターンに news.replied を書く。\n" +
	"ラベル role=chief-of-staff の無い端末では書かずに終了コード 1。\n"

// AckLabel は Chief of Staff の「人に言うことが無い」の申告（contract.md §8）。
const AckLabel = "news.ack"

// NewsMain は andon news。
func NewsMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "ack" {
		fmt.Fprint(stderr, NewsUsage)
		return exitUsage
	}
	addr := os.Getenv("HC_TERMINAL_ID")
	if addr == "" {
		fmt.Fprintln(stderr, "andon news ack: HC_TERMINAL_ID が無い（hc の端末の中で打つ）")
		return exitFail
	}
	t, err := hcGet(addr)
	if err != nil {
		fmt.Fprintf(stderr, "andon news ack: %v\n", err)
		return exitFail
	}
	if t.Labels["role"] != "chief-of-staff" {
		fmt.Fprintf(stderr, "andon news ack: %s は Chief of Staff の端末ではない（ラベル role=chief-of-staff が無い）。書かない\n", addr)
		return exitFail
	}
	cmd := exec.Command("hc", "label", addr, AckLabel+"="+strconv.Itoa(t.Turn))
	cmd.Env = withoutTerminal()
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(stderr, "andon news ack: hc label: %v %s\n", err, strings.TrimSpace(string(out)))
		return exitFail
	}
	fmt.Fprintf(stdout, "%s に %s=%d を書いた。文を書かずにターンを終える\n", addr, AckLabel, t.Turn)
	return 0
}

// acked は、端末 t のラベル news.ack が、<news> を渡したときに控えた turn と同じか（ADR-0010 決めたこと 5）。
// before は渡したときのラベルの値で、渡す前からその値なら（同じ turn の前の ack）数えない。
func acked(t *terminal, turn *int, before string) bool {
	if t == nil || turn == nil {
		return false
	}
	v, ok := t.Labels[AckLabel]
	return ok && v == strconv.Itoa(*turn) && v != before
}

// countReplied は news.replied を何で書いたか（ack・no-reply）を控えに数える（移行の間。ADR-0010 決めたこと 6）。
func (s *State) countReplied(by string) {
	if s.Replied == nil {
		s.Replied = map[string]int{}
	}
	s.Replied[by]++
}
