// Package gate は andon gate（docs/contract.md §7。仮設のゲート lib/yard/gate を Go に移したもの）。
// 台帳のタスクの PR を、task.yaml の予約キー・report.json の pr と acceptance・レビュアーの findings.json・
// ゲートの条件表・委譲（delegation.json）で照合し、decision（approve / fix / needs_human）を出して記録する。
// Markdown（依頼書・報告）は読まない。マージはしない（クーリエが記録を読んでする。P6a）。
//
// 終了コード: approve 0・fix 1・使い方 2・道具や台帳の誤り 3・needs_human 4。
package gate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/ledger"
)

// Usage は andon gate --help の中身。
const Usage = "usage: andon gate <id> [--dry-run]\n" +
	"台帳のタスクの PR（report.json の pr）を、task.yaml の予約キー・findings.json・条件表・委譲で照合し、記録を JSON で出す。\n" +
	"記録は tasks/<id>/library/gate/<head>.json に書き、PR に commit status（andon/gate）とコメントを付ける（--dry-run は書かない）。\n" +
	"マージはしない。終了コード: approve 0・fix 1・needs_human 4・誤り 3。\n"

const (
	exitFix   = 1
	exitUsage = 2
	exitError = 3
	exitHuman = 4
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Record はゲートの記録（schema/gate.schema.json の record）。
type Record struct {
	At           string        `json:"at"`
	ID           string        `json:"id"`
	PR           string        `json:"pr"`
	Head         string        `json:"head"`
	Tier         string        `json:"tier"`
	TierReasons  []string      `json:"tier_reasons"`
	ChangedLines int           `json:"changed_lines"`
	Checks       []Check       `json:"checks"`
	Review       ledger.Review `json:"review"`
	Acceptance   struct {
		Pass   int `json:"pass"`
		Fail   int `json:"fail"`
		Manual int `json:"manual"`
	} `json:"acceptance"`
	Delegation struct {
		Merge string `json:"merge"`
		OK    bool   `json:"ok"`
	} `json:"delegation"`
	Decision string   `json:"decision"`
	Reasons  []string `json:"reasons"`
	Merged   bool     `json:"merged"`
}

type failure struct{ msg string }

func (f failure) Error() string { return f.msg }

func fail(format string, a ...any) error { return failure{fmt.Sprintf(format, a...)} }

// Main は andon gate。
func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 || len(args) > 2 || !idRe.MatchString(args[0]) || (len(args) == 2 && args[1] != "--dry-run") {
		fmt.Fprint(stderr, Usage)
		return exitUsage
	}
	dry := len(args) == 2
	rec, early, err := run(args[0], dry, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "andon gate: %v\n", err)
		return exitError
	}
	if early != "" {
		// 信じられる PR が無い（report.json・台帳と PR が合わない）。記録も GitHub への書き込みもしない。
		fmt.Fprintf(stdout, "{\"decision\": \"fix\", \"reasons\": [%q]}\n", early)
		return exitFix
	}
	stdout.Write(contract.Marshal(rec))
	switch rec.Decision {
	case "approve":
		return 0
	case "fix":
		return exitFix
	}
	return exitHuman
}

// gh は gh を打ち、stdout を返す。
func gh(args ...string) ([]byte, error) {
	var errb bytes.Buffer
	c := exec.Command("gh", args...)
	c.Stderr = &errb
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %v %s", strings.Join(args[:min(2, len(args))], " "), err, strings.TrimSpace(errb.String()))
	}
	return out, nil
}

type prView struct {
	State             string `json:"state"`
	IsDraft           bool   `json:"isDraft"`
	HeadRefName       string `json:"headRefName"`
	HeadRefOid        string `json:"headRefOid"`
	BaseRefName       string `json:"baseRefName"`
	MergeStateStatus  string `json:"mergeStateStatus"`
	ChangedFiles      int    `json:"changedFiles"`
	IsCrossRepository bool   `json:"isCrossRepository"`
	Author            struct {
		Login string `json:"login"`
	} `json:"author"`
}

// LoadTable は条件表を読む: パッケージの gate.json に、建屋の gate.local.json のキーを1つずつ置き換えて重ねる。
// gate.local.json は建屋の repo の HEAD のもので、最後に変えたのが owner のときだけ重ねる（委譲と同じ読み方。
// 信じられなければ重ねず、理由を warn に返す。repos が無くなるのでゲートは needs_human に倒れる）。
func LoadTable(pkg, ws string) (*Table, string, error) {
	merged := map[string]json.RawMessage{}
	add := func(b []byte, p string) error {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("条件表が JSON でない: %s", p)
		}
		for k, v := range m {
			merged[k] = v
		}
		return nil
	}
	b, err := os.ReadFile(filepath.Join(pkg, "gate.json"))
	if err != nil {
		return nil, "", fmt.Errorf("条件表が読めない: %v", err)
	}
	if err := add(b, "gate.json"); err != nil {
		return nil, "", err
	}
	warn := ""
	switch local, err := delegate.OwnerFile(ws, "gate.local.json"); {
	case errors.Is(err, delegate.ErrAbsent):
	case err != nil:
		warn = "gate.local.json を重ねない: " + err.Error()
	default:
		if err := add(local, "gate.local.json"); err != nil {
			return nil, "", err
		}
	}
	b, _ = json.Marshal(merged)
	var t Table
	if err := contract.Decode(b, &t); err != nil {
		return nil, "", fmt.Errorf("条件表の形が違う: %v", err)
	}
	for r, rt := range t.Repos {
		if rt.Merge != "" && rt.Merge != "merge" && rt.Merge != "squash" && rt.Merge != "rebase" {
			return nil, "", fmt.Errorf("条件表の repos.%s.merge が知らない値: %q", r, rt.Merge)
		}
		for g, v := range rt.TierOverrides {
			if v != "A" && v != "B" && v != "C" {
				return nil, "", fmt.Errorf("条件表の repos.%s.tier_overrides[%s] が A/B/C でない", r, g)
			}
		}
	}
	if t.SizeHumanLines == nil || t.SizeReviewLines == nil {
		return nil, "", errors.New("条件表に size_review_lines・size_human_lines が無い")
	}
	return &t, warn, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func run(id string, dry bool, stderr io.Writer) (*Record, string, error) {
	for _, c := range []string{ledger.CLI(), "gh", "git"} {
		if _, err := exec.LookPath(c); err != nil {
			return nil, "", fmt.Errorf("%s が無い", c)
		}
	}
	l, err := ledger.Open()
	if err != nil {
		return nil, "", err
	}
	t, err := l.Show(id)
	if err != nil {
		return nil, "", err
	}
	ws := delegate.Workspace()
	table, warn, err := LoadTable(getenv("ANDON_HOME", "/opt/andon"), ws)
	if err != nil {
		return nil, "", err
	}
	if warn != "" {
		fmt.Fprintln(stderr, "andon gate:", warn)
	}

	// L1 台帳: PR の場所は report.json の pr だけ（Markdown も task.yaml の pr も読まない）
	rep, err := contract.ReadReport(filepath.Join(l.Library(id), "report.json"))
	if err != nil {
		return nil, "report.json が無い・形が違う（andon task report）", nil
	}
	owner, name, number, ok := contract.ParsePR(rep.PR)
	if !ok {
		return nil, "report.json に pr が無い（andon task report --pr）", nil
	}
	repo := t.Str("repo")
	switch {
	case repo != "github.com/"+owner+"/"+name:
		return nil, fmt.Sprintf("task.yaml の repo（%s）と PR の repo（%s/%s）が違う", repo, owner, name), nil
	case !contains(l.Repos(), repo):
		return nil, "台帳の config.yaml の repos にこの repo が無い", nil
	case t.Str("branch") == "":
		return nil, "task.yaml に branch が無い", nil
	}
	slug := owner + "/" + name

	var pv prView
	b, err := gh("pr", "view", number, "--repo", slug, "--json", "state,isDraft,headRefName,headRefOid,baseRefName,mergeStateStatus,author,changedFiles,isCrossRepository")
	if err != nil {
		return nil, "", err
	}
	if json.Unmarshal(b, &pv) != nil || !contract.IsSHA(pv.HeadRefOid) {
		return nil, "", errors.New("PR が読めない")
	}
	head := pv.HeadRefOid
	rec := &Record{At: time.Now().UTC().Format("2006-01-02T15:04:05Z"), ID: id, PR: rep.PR, Head: head, Checks: []Check{}, TierReasons: []string{}}
	facts := Facts{PROpen: pv.State == "OPEN" && !pv.IsDraft}
	if t.Str("status") != "review" {
		facts.LedgerReasons = append(facts.LedgerReasons, fmt.Sprintf("台帳のタスクが review でない（%s）", t.Str("status")))
	}
	if pv.HeadRefName != t.Str("branch") {
		facts.LedgerReasons = append(facts.LedgerReasons, fmt.Sprintf("PR の branch（%s）が task.yaml の branch と違う", pv.HeadRefName))
	}
	if rep.Commit != head {
		facts.LedgerReasons = append(facts.LedgerReasons, "report.json の commit が PR の head と違う（andon task ready からやり直す）")
	}

	files, err := prFiles(slug, number)
	if err != nil {
		return nil, "", err
	}
	diff, err := gh("pr", "diff", number, "--repo", slug)
	if err != nil {
		return nil, "", err
	}
	def, err := gh("api", "repos/"+slug, "--jq", ".default_branch")
	if err != nil {
		return nil, "", err
	}
	actor, err := gh("api", "user", "--jq", ".login")
	if err != nil {
		return nil, "", err
	}
	if len(files) != pv.ChangedFiles {
		facts.Blockers = append(facts.Blockers, "files の件数が changedFiles と合わない（一覧の上限・取得中の変更）")
	}
	if pv.MergeStateStatus == "DIRTY" {
		facts.Blockers = append(facts.Blockers, "conflict")
	}
	if pv.BaseRefName != strings.TrimSpace(string(def)) {
		facts.Blockers = append(facts.Blockers, "base が既定ブランチでない")
	}
	if pv.Author.Login != strings.TrimSpace(string(actor)) {
		facts.Blockers = append(facts.Blockers, "作者が建屋の gh アカウントでない")
	}
	if pv.IsCrossRepository {
		facts.Blockers = append(facts.Blockers, "fork の PR")
	}

	// 予約キーは Chief of Staff が書いたものだけを信じる（log.jsonl の by）
	rows := l.Log()
	s := ledger.SettingsOf(t)
	switch {
	case s.Review == "" || s.MustFix == "":
		facts.SettingsReason = "task.yaml の review・must_fix が無い・知らない値（Chief of Staff が kanban set する）"
	case !l.Trusted(rows, t, "review") || !l.Trusted(rows, t, "must_fix"):
		facts.SettingsReason = "task.yaml の review・must_fix を最後に書いたのが Chief of Staff（config.yaml の coordinator）でない・記録と値が違う"
	}
	if s.Merge == "auto" && !l.Trusted(rows, t, "merge") {
		s.Merge = "person" // 信じられない auto は無いのと同じ（既定 person）
	}

	rt, known := table.Repos[repo]
	facts.RepoKnown = known
	cl, err := table.Classify(rt, files, string(diff), s.Review == "on")
	if err != nil {
		return nil, "", err
	}
	for _, a := range rep.Acceptance {
		switch a.Result {
		case "pass":
			rec.Acceptance.Pass++
		case "fail":
			rec.Acceptance.Fail++
		case "manual":
			rec.Acceptance.Manual++
		}
	}
	tier := cl.Tier
	if rec.Acceptance.Manual > 0 && tier != "C" {
		tier = Raise(tier)
		cl.TierReasons = append(cl.TierReasons, fmt.Sprintf("受け入れ条件の manual が %d 件（1つ上げた）", rec.Acceptance.Manual))
	}
	facts.Tier, facts.AcceptanceFail = tier, rec.Acceptance.Fail
	rec.Tier, rec.TierReasons, rec.ChangedLines = tier, cl.TierReasons, cl.ChangedLines

	d := delegate.Read(ws, l.Slug, time.Now().UTC().Format("2006-01-02"))
	facts.MergeLevel = d.Merge()
	rec.Delegation.Merge, rec.Delegation.OK = facts.MergeLevel, Within(tier, facts.MergeLevel)

	// files・diff は head と別に取っている。取り直して動いていれば判定しない
	b, err = gh("pr", "view", number, "--repo", slug, "--json", "headRefOid", "-q", ".headRefOid")
	if err != nil {
		return nil, "", err
	}
	if after := strings.TrimSpace(string(b)); after != head {
		return nil, fmt.Sprintf("判定の途中で PR の head が動いた（%s → %s）。取り直す", head, after), nil
	}

	// L3 レビュー・L2 検査（前の段で決まっていれば回さない）
	facts.ReviewRequired = tier == "B" || tier == "C" || s.Review == "on"
	rec.Review = ledger.Review{State: "off", Waived: len(rep.Waived)}
	gated := len(facts.LedgerReasons) == 0 && facts.PROpen && len(facts.Blockers) == 0 && facts.SettingsReason == "" && known && rec.Delegation.OK
	if facts.ReviewRequired {
		tasks, err := l.List()
		if err != nil {
			return nil, "", err
		}
		rec.Review = l.CheckReview(rows, tasks, t, head, s.MustFix, rep.Waived)
	}
	facts.Review = rec.Review
	if gated && (!facts.ReviewRequired || rec.Review.State == "current") {
		rec.Checks, err = runChecks(repo, head, rt.Check)
		if err != nil {
			return nil, "", err
		}
	}
	facts.Checks = rec.Checks
	path := filepath.Join(l.Library(id), "gate", head+".json")
	if prev, err := os.ReadFile(path); err == nil {
		var p Record
		if json.Unmarshal(prev, &p) == nil && p.Decision == "fix" && p.Review.MustFixOpen > 0 {
			facts.PreviousMustFix = true
		}
	}
	rec.Decision, rec.Reasons = Decide(facts)
	if dry {
		return rec, "", nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, "", err
	}
	if err := contract.Write(path, rec); err != nil {
		return nil, "", err
	}
	postGitHub(slug, number, rec, stderr)
	return rec, "", nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// prFiles は PR の変更されたファイル（REST を全ページ。GraphQL の files は100件で切れ、rename 元も無い）。
func prFiles(slug, number string) ([]File, error) {
	b, err := gh("api", "--paginate", "--slurp", "repos/"+slug+"/pulls/"+number+"/files")
	if err != nil {
		return nil, err
	}
	var pages [][]struct {
		Filename         string `json:"filename"`
		PreviousFilename string `json:"previous_filename"`
		Additions        int    `json:"additions"`
		Deletions        int    `json:"deletions"`
	}
	if err := json.Unmarshal(b, &pages); err != nil {
		return nil, errors.New("PR の files が読めない")
	}
	files := []File{}
	for _, p := range pages {
		for _, f := range p {
			files = append(files, File{f.Filename, f.PreviousFilename, f.Additions, f.Deletions})
		}
	}
	return files, nil
}

// runChecks は条件表の check を PR の head の一時 worktree で順に回す（最初の失敗で止める）。
// clone は GHQ_ROOT（無ければ ghq root）の下の repo。
func runChecks(repo, head string, cmds []string) ([]Check, error) {
	checks := []Check{}
	if len(cmds) == 0 {
		return checks, nil
	}
	root := os.Getenv("GHQ_ROOT")
	if root == "" {
		out, err := exec.Command("ghq", "root").Output()
		if err != nil {
			return nil, errors.New("GHQ_ROOT が無い")
		}
		root = strings.TrimSpace(string(out))
	}
	dir := filepath.Join(root, repo)
	scratch, err := os.MkdirTemp("", "andon-gate-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	wt := filepath.Join(scratch, "worktree")
	if out, err := exec.Command("git", "-C", dir, "fetch", "-q", "https://"+repo, head).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("PR の head の fetch に失敗: %s", strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("git", "-C", dir, "worktree", "add", "-q", "--detach", wt, head).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("一時 worktree を作れない: %s", strings.TrimSpace(string(out)))
	}
	defer exec.Command("git", "-C", dir, "worktree", "remove", "--force", wt).Run()
	tmp := filepath.Join(scratch, "tmp")
	if err := os.Mkdir(tmp, 0o755); err != nil {
		return nil, err
	}
	for _, cmd := range cmds {
		c := exec.Command("bash", "-e", "-o", "pipefail", "-c", cmd)
		c.Dir, c.Env = wt, append(os.Environ(), "TMPDIR="+tmp)
		ok := c.Run() == nil
		checks = append(checks, Check{Cmd: cmd, OK: ok})
		if !ok {
			break
		}
	}
	return checks, nil
}

// postGitHub は記録を PR の commit status（andon/gate）とコメントに写す。失敗しても判定は変えない。
func postGitHub(slug, number string, rec *Record, stderr io.Writer) {
	state := "failure"
	if rec.Decision == "approve" {
		state = "success"
	}
	desc := "andon gate: " + rec.Decision + "（tier " + rec.Tier + "）"
	if _, err := gh("api", "-X", "POST", "repos/"+slug+"/statuses/"+rec.Head, "-f", "context=andon/gate", "-f", "state="+state, "-f", "description="+desc); err != nil {
		fmt.Fprintln(stderr, "andon gate: commit status の記録に失敗:", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "andon gate: **%s**（commit %s）\n\n", rec.Decision, rec.Head)
	fmt.Fprintf(&b, "- tier %s（%s）・%d 行\n", rec.Tier, strings.Join(rec.TierReasons, "、"), rec.ChangedLines)
	for _, c := range rec.Checks {
		fmt.Fprintf(&b, "- check `%s`: %s\n", c.Cmd, strconv.FormatBool(c.OK))
	}
	fmt.Fprintf(&b, "- review %s（Must fix %d・見送り %d）\n", rec.Review.State, rec.Review.MustFixOpen, rec.Review.Waived)
	fmt.Fprintf(&b, "- delegation merge=%s（%v）\n", rec.Delegation.Merge, rec.Delegation.OK)
	for _, r := range rec.Reasons {
		fmt.Fprintf(&b, "- %s\n", r)
	}
	if _, err := gh("pr", "comment", number, "--repo", slug, "--body", b.String()); err != nil {
		fmt.Fprintln(stderr, "andon gate: コメントの記録に失敗:", err)
	}
}
