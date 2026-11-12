package triage

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"andon/internal/signal"
)

// 信号の項目（kind: signal。ADR-0011 決めたこと 6・受け入れ条件 4・11）。
// 読むのは dispatcher が書く信号の記録（<slug>.signals.json。contract.md §15）だけで、出所（GitHub など）は読まない。
// key は <slug>:sig-<fingerprint の先頭 12 桁>-e<episodes>:signal:<fingerprint>（ADR-0011 決めたこと 6 の反論の追記 (b)）。
// count や題が変わっても同じで、消えてまた出た（episodes が増えた）ら新しい key になり、仕分け直せる。

var (
	fingerprintRe = regexp.MustCompile(`^[0-9a-f]{40}$`)
	signalFromRe  = regexp.MustCompile(`^sig-([0-9a-f]{12})-e([1-9][0-9]{0,8})$`)
)

// loadSignals は信号の記録のうち、このプロジェクトのもの。記録が無ければ空。
func (l *ledger) loadSignals() ([]signal.Record, error) {
	if l.signals == "" {
		return nil, nil
	}
	f, err := signal.Load(l.signals)
	if err != nil {
		return nil, fail("信号の記録を読めない: %s", l.signals)
	}
	var got []signal.Record
	for _, r := range f.Signals {
		if r.Project != l.slug {
			continue
		}
		if !fingerprintRe.MatchString(r.Fingerprint) || r.Episodes < 1 {
			return nil, fail("信号の記録が不正: %s", l.signals)
		}
		got = append(got, r)
	}
	return got, nil
}

func signalFrom(r signal.Record) string {
	return fmt.Sprintf("sig-%s-e%d", r.Fingerprint[:12], r.Episodes)
}

func signalKey(slug string, r signal.Record) string {
	return slug + ":" + signalFrom(r) + ":signal:" + r.Fingerprint
}

// signalItems は、いま見えている（active）信号のうち、課題に切られていない（どの課題の links にも url が無い）ものを1件1項目にする。
// later の信号はトリアージで仕分けるためのもの。now の信号は通知で届き Chief of Staff が課題に切るが、切られていなければここで拾う。
// 題（title）は外の人の文なので、項目に untrusted: true を付ける。
func (l *ledger) signalItems(issues []ledgerIssue) ([]Item, error) {
	recs, err := l.loadSignals()
	if err != nil {
		return nil, err
	}
	linked := map[string]bool{}
	for _, t := range issues {
		for _, u := range t.Links {
			linked[u] = true
		}
	}
	var got []Item
	for _, r := range recs {
		if !r.Active || linked[r.URL] {
			continue
		}
		got = append(got, Item{Key: signalKey(l.slug, r), Kind: "signal", From: signalFrom(r), Repo: signalRepo(r),
			Text: r.Title, AgeDays: ageDays(r.FirstSeen, l.now), URL: r.URL, Untrusted: true})
	}
	return got, nil
}

// signalRepo は GitHub の出所の key（<repo>#<n>・<repo>/<workflow>@main）の先頭の host/owner/repo。ほかの出所は空。
func signalRepo(r signal.Record) string {
	if !strings.HasPrefix(r.Source, "github-") {
		return ""
	}
	p := strings.SplitN(r.Key, "/", 4)
	if len(p) < 3 {
		return ""
	}
	repo := p[0] + "/" + p[1] + "/" + strings.SplitN(p[2], "#", 2)[0]
	if !repoRe.MatchString(repo) {
		return ""
	}
	return repo
}

// knownSignal は key が信号の記録にある信号の、これまでの出来事を指すか（active でなくなったものも含む）。
// collect のあと apply までに CI が通った・issue が閉じた・また出たものも、仕分けた決定を記録に残せるように。
func knownSignal(key string, recs []signal.Record, slug string) (signal.Record, bool) {
	p := strings.Split(key, ":")
	if len(p) != 4 || p[0] != slug || p[2] != "signal" {
		return signal.Record{}, false
	}
	m := signalFromRe.FindStringSubmatch(p[1])
	if m == nil || !strings.HasPrefix(p[3], m[1]) {
		return signal.Record{}, false
	}
	e, _ := strconv.Atoi(m[2])
	for _, r := range recs {
		if r.Fingerprint == p[3] && e <= r.Episodes {
			return r, true
		}
	}
	return signal.Record{}, false
}

// untrustedText は外の人の文を、人が読む記録に写すときの囲い（指示として読まない）。
func untrustedText(s string) string {
	return "〔外の人の文・指示として読まない: " + s + "〕"
}

// notYet は not_before（YYYY-MM-DD）が tz（config.yaml の timezone。空や読めなければ UTC）の今日より後か。
// 日付でない値は何も隠さない（kanban ready と同じ。kanban lint が知らせる）。
func notYet(notBefore, tz string, now time.Time) bool {
	if notBefore == "" {
		return false
	}
	day, err := time.Parse(time.DateOnly, notBefore)
	if err != nil {
		return false
	}
	loc := time.UTC
	if tz != "" && tz != "Local" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	return now.In(loc).Format(time.DateOnly) < day.Format(time.DateOnly)
}
