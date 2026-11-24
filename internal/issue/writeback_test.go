package issue

import (
	"slices"
	"testing"

	"andon/internal/delegate"
)

func TestTrackers(t *testing.T) {
	yes, no := true, false
	app, other := "github.com/example/app", "github.com/example/other"
	p := &delegate.Project{Repos: []string{app}, References: []string{other}, Signals: []delegate.Signal{
		{Source: "github-issues", Weight: "later", WriteBack: &yes},
		{Source: "github-ci", Weight: "now", WriteBack: &yes},
	}}
	links := []string{
		"https://github.com/example/app/issues/3",
		"https://github.com/example/app/issues/3",
		"https://github.com/example/app/pull/4",
		"https://github.com/example/other/issues/5",
		"https://github.com/example/app/issues/3/x",
		"http://github.com/example/app/issues/6",
		"https://linear.app/example/issue/S-1",
		"docs/a.md",
	}
	if got := trackers(p, links); !slices.Equal(got, []string{"https://github.com/example/app/issues/3"}) {
		t.Errorf("repo の無い出所: %v", got)
	}
	p.Signals[0].Repo = &other
	if got := trackers(p, links); !slices.Equal(got, []string{"https://github.com/example/other/issues/5"}) {
		t.Errorf("出所の repo を書いたとき: %v", got)
	}
	p.Signals[0].WriteBack = &no
	if got := trackers(p, links); len(got) != 0 {
		t.Errorf("write_back: false で書く: %v", got)
	}
	p.Signals[0].WriteBack = nil
	if got := trackers(p, links); len(got) != 0 {
		t.Errorf("write_back が無いのに書く: %v", got)
	}
}

// ADR-0026 A7: 外の Issue への本文は 3 つの場合で違い、どれにも課題番号が出ない
func TestWriteBackBody(t *testing.T) {
	pr := "https://github.com/example/app/pull/7"
	cases := map[string]string{
		resultMerged: "この Issue に対応する変更がマージされた。\nPR: " + pr + "\n",
		resultClosed: "この Issue に対応する変更は、マージされずに閉じられた。\nPR: " + pr + "\n",
		resultNoPR:   "この Issue に対応する作業は、変更（PR）を作らずに終えた。\n",
	}
	for result, want := range cases {
		p := pr
		if result == resultNoPR {
			p = ""
		}
		if got := writeBackBody(p, result); got != want {
			t.Errorf("%s: %q want %q", result, got, want)
		}
	}
}
