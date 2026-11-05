// Package chiefengineer は andon chief-engineer: 人と仕様・設計を対話する Chief Engineer を
// 台帳のタスクとして起こす（docs/design.md §1.8）。
//
//	andon chief-engineer "<題>"   projects/<slug>/ で打つ
//
// することは3つだけ: 台帳にタスクを足す（kanban add）・端末の土台でプロファイル chief-engineer を
// 起こす（hc run）・タスクを doing にして agent をその端末にする（kanban start）。台帳の CLI は今は deck。
// 1プロジェクトに Chief Engineer は1人（WIP 制限）なので、動いていれば起こさずに止まる。
package chiefengineer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Usage は andon chief-engineer の使い方。
const Usage = "usage: andon chief-engineer \"<題>\"\n" +
	"projects/<slug>/ で打つ。台帳にタスクを足し（プロファイル chief-engineer）、hc で Chief Engineer を起こし、\n" +
	"タスクを doing にする。Chief Engineer が動いていれば起こさずに止まる（1プロジェクトに1人）。\n"

// Profile は Chief Engineer のプロファイル（hc の template の名前）。
const Profile = "chief-engineer"

const (
	exitFail  = 1
	exitUsage = 2
	exitFirst = 3 // 起きたが、最初のメッセージが届いていない
)

var (
	slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	idRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	nameRe = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
)

// Main は andon chief-engineer を走らせ、終了コードを返す。
func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" || strings.ContainsAny(args[0], "\r\n") || strings.HasPrefix(args[0], "-") {
		fmt.Fprint(stderr, Usage)
		return exitUsage
	}
	title := strings.TrimSpace(args[0])
	die := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "andon chief-engineer: "+format+"\n", a...)
		return exitFail
	}

	for _, c := range []string{"deck", "hc"} {
		if _, err := exec.LookPath(c); err != nil {
			return die("%s が無い", c)
		}
	}

	root, err := output(nil, "", "deck", "root")
	if err != nil || root == "" {
		return die("台帳が見つからない（projects/<slug>/ で打つ）")
	}
	slug := filepath.Base(root)
	if !slugRe.MatchString(slug) {
		return die("slug が不正: '%s'（台帳の root のフォルダ名。英小文字・数字・-）", slug)
	}
	env := []string{"FLIGHTDECK_ROOT=" + root}

	// 1プロジェクトに1人
	ls, err := output(nil, "", "hc", "ls", "--json", "--label", "role="+Profile, "--label", "project="+slug)
	if err != nil {
		return die("hc ls に失敗: %v", err)
	}
	var live struct {
		Terminals []struct {
			Address string            `json:"address"`
			Labels  map[string]string `json:"labels"`
		} `json:"terminals"`
	}
	if err := json.Unmarshal([]byte(ls), &live); err != nil {
		return die("hc ls の返事が読めない: %v", err)
	}
	if len(live.Terminals) > 0 {
		t := live.Terminals[0]
		return die("Chief Engineer が動いている: %s（タスク %s）。話すには hc attach %s。新しい題は、前の題を渡してから", t.Address, t.Labels["task"], t.Address)
	}

	tpl, err := output(nil, "", "hc", "templates", "--json")
	if err != nil {
		return die("hc templates に失敗: %v", err)
	}
	var tpls struct {
		Templates []struct {
			Name string `json:"name"`
		} `json:"templates"`
	}
	if err := json.Unmarshal([]byte(tpl), &tpls); err != nil {
		return die("hc templates の返事が読めない: %v", err)
	}
	found := false
	for _, t := range tpls.Templates {
		found = found || t.Name == Profile
	}
	if !found {
		return die("プロファイル %s の template が無い（hc templates で確かめる。作るのは人）", Profile)
	}

	brief := "# " + title + "\n\n" +
		"Chief Engineer のタスク。人と直接話して仕様・設計を詰め、文書にして Chief of Staff へ渡す。\n" +
		"題は人の1行。手順は skill chief-engineer。\n"
	id, err := output(env, brief, "deck", "add", title, "--profile", Profile, "--brief", "-")
	if err != nil || !idRe.MatchString(id) {
		return die("deck add に失敗: %v %q", err, id)
	}

	name := strings.ToLower(slug + "-" + id)
	if !nameRe.MatchString(name) {
		return die("端末の名前 %s が hc の決まり（a-z 0-9 - _ で32字まで）に合わない。タスク %s は todo のまま残る", name, id)
	}
	cwd := filepath.Join(root, ".flightdeck", "tasks", id)
	msg := fmt.Sprintf("タスク `%s/%s`。\n依頼書は `%s`。\n先に `%s` を読む。",
		slug, id, filepath.Join(cwd, "brief.md"), filepath.Join(home(), "skills", "chief-engineer", "SKILL.md"))

	// --no-notify: 止まった・終わったはクーリエが端末の土台から拾う（contract.md §8）
	out, err := output(nil, "", "hc", "run", "--json", "--bg", "--outlive-parent", "--no-notify", "--name", name,
		"--label", "role="+Profile, "--label", "project="+slug, "--label", "task="+id, "--label", "title="+title,
		"--cwd", cwd, Profile, msg)
	if err != nil {
		return die("hc run に失敗: %v。タスク %s は todo のまま残る", err, id)
	}
	var run struct {
		Address      string `json:"address"`
		FirstMessage struct {
			Sent   bool   `json:"sent"`
			Reason string `json:"reason"`
		} `json:"first_message"`
	}
	if err := json.Unmarshal([]byte(out), &run); err != nil || run.Address == "" {
		return die("hc run の返事に address が無い: %s。タスク %s は todo のまま残る", out, id)
	}

	if _, err := output(append(env, "HC_TERMINAL_ID="+run.Address), "", "deck", "start", id); err != nil {
		return die("deck start %s に失敗: %v（端末 %s は起きている）", id, err, run.Address)
	}

	if !run.FirstMessage.Sent {
		reason := run.FirstMessage.Reason
		if reason == "" {
			reason = "理由なし"
		}
		fmt.Fprintf(stderr, "andon chief-engineer: %s（%s）に最初のメッセージが届いていない: %s。hc attach %s で画面に答えてから、次を送る: hc send %s %q\n",
			run.Address, name, reason, run.Address, run.Address, msg)
		return exitFirst
	}
	fmt.Fprintf(stdout, "andon chief-engineer: %s（%s）を %s（%s）で起こした。話すには hc attach %s\n", id, title, run.Address, name, run.Address)
	return 0
}

// home はパッケージの置き場。ANDON_HOME（checkout の package/ を指して試す）が無ければ /opt/andon（design.md §2.5）。
func home() string {
	if h := os.Getenv("ANDON_HOME"); h != "" {
		return h
	}
	return "/opt/andon"
}

// output はコマンドを打ち、stdout を返す（前後の空白を除く）。stderr は失敗のときだけ err に入れる。
func output(env []string, stdin, name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	if env != nil {
		c.Env = append(os.Environ(), env...)
	}
	c.Stdin = strings.NewReader(stdin)
	var errb bytes.Buffer
	c.Stderr = &errb
	b, err := c.Output()
	if err != nil {
		if s := strings.TrimSpace(errb.String()); s != "" {
			return "", fmt.Errorf("%w: %s", err, firstLine(s))
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
