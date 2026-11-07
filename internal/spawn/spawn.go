// Package spawn はエージェントを起こす command（andon chief-of-staff・andon chief-engineer・andon issue start）が
// 共有するもの: コマンドを打つ・repo の置き場・worktree と branch を切る・hc の template を確かめる・hc run を打つ。
//
// 置き場（docs/contract.md §8）:
//
//	worktree  ~/.herdr/worktrees/<repo 名>/<slug>-<id>
//	branch    <slug>/<id>-<題の英字の短い形>。origin/<既定の branch> から --no-track で切る
//	repo      $GHQ_ROOT（無ければ ghq root）/<host/owner/repo>
package spawn

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// RepoRe は台帳の repo（<host>/<owner>/<repo>）。
	RepoRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	// NameRe は hc の端末の名前の決まり（a-z 0-9 - _ で32字まで）。
	NameRe = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
	// SlugRe は台帳の root のフォルダ名（schema の project と同じ）。
	SlugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

// Output はコマンドを打ち、stdout を返す（前後の空白を除く）。stderr は失敗のときだけ err に入れる。
func Output(env []string, stdin, name string, args ...string) (string, error) {
	return OutputIn("", env, stdin, name, args...)
}

// OutputIn は dir で Output する。env は今の環境に足す（nil なら今の環境のまま）。
func OutputIn(dir string, env []string, stdin, name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	c.Dir = dir
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

// RepoPath は $GHQ_ROOT（無ければ ghq root）/<repo>。
func RepoPath(repo string) (string, error) {
	root := os.Getenv("GHQ_ROOT")
	if root == "" {
		if out, err := Output(nil, "", "ghq", "root"); err == nil {
			root = out
		}
	}
	if root == "" {
		return "", fmt.Errorf("GHQ_ROOT が無い")
	}
	return filepath.Join(root, repo), nil
}

// HasTemplate は hc の template に name があるか（hc templates --json）。
func HasTemplate(name string) (bool, error) {
	out, err := Output(nil, "", "hc", "templates", "--json")
	if err != nil {
		return false, fmt.Errorf("hc templates に失敗: %v", err)
	}
	var tpls struct {
		Templates []struct {
			Name string `json:"name"`
		} `json:"templates"`
	}
	if err := json.Unmarshal([]byte(out), &tpls); err != nil {
		return false, fmt.Errorf("hc templates の返事が読めない: %v", err)
	}
	for _, t := range tpls.Templates {
		if t.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// Started は hc run --json の返事。
type Started struct {
	Address string `json:"address"`
	First   First  `json:"first_message"`
}

// First は hc run の first_message。Started は送ったあと数秒で working になったか
// （null は状態を報告しない program で、誰にも分からない。hc 0.1.17 の terminal.send）。
type First struct {
	Sent    bool   `json:"sent"`
	Started *bool  `json:"started"`
	Reason  string `json:"reason"`
}

// Delivered は最初のメッセージが届いたか: 送れて、動き出さなかった（started=false）のでもない。
// 送れても動き出さなければ、起動の途中で入力を捨てられたとみる（Codex 0.166.0。dispatcher.md §5）。
func (f First) Delivered() bool {
	return f.Sent && (f.Started == nil || *f.Started)
}

// Reason は最初のメッセージが届かなかった理由（無ければ「理由なし」）。
func (s *Started) Reason() string {
	switch {
	case s.First.Sent && !s.First.Delivered():
		return "送ったが動き出さなかった（起動の途中で入力を捨てられたとみる）"
	case s.First.Reason == "":
		return "理由なし"
	}
	return s.First.Reason
}

// FirstLabel は最初のメッセージが届かなかった端末に付けるラベルの名前。値は "false"。
// dispatcher が見回りで拾い、端末が入力を受けられるようになったら最初のメッセージを送って外す（contract.md §8・dispatcher.md §5）。
const FirstLabel = "first_message.sent"

// FirstMessage は担当（andon issue start）と Chief Engineer（andon chief-engineer）の最初のメッセージ（課題の2行）。
// skill はプロファイルの command が system prompt に足すので、ここには書かない（ADR-0001 決定 3）。
// dispatcher が送り直すときも同じ文を組む。brief は依頼書の絶対パス。
func FirstMessage(slug, id, brief string) string {
	return fmt.Sprintf("課題 `%s/%s`。\n依頼書は `%s`。", slug, id, brief)
}

// MarkUnsent は端末に FirstLabel=false を付ける（dispatcher に送り直しを預ける）。
func MarkUnsent(addr string) error {
	_, err := Output(nil, "", "hc", "label", addr, FirstLabel+"=false")
	return err
}

// ResendHint は最初のメッセージが届かなかった端末 addr に FirstLabel を付け、人か Chief of Staff への案内を返す。
// 送り直しは dispatcher に預ける（画面が入力を受けられるようになれば次の見回りで送る）。付けられなければ手で送る案内だけにする。
func ResendHint(addr, msg string) string {
	if err := MarkUnsent(addr); err != nil {
		return fmt.Sprintf("ラベル %s を付けられず（%v）、dispatcher は送り直さない。hc read %s で画面を読み、答えてから手で送る: hc send %s %q", FirstLabel, err, addr, addr, msg)
	}
	return fmt.Sprintf("dispatcher が見回りで送り直す（画面が入力を受けられるようになれば。信頼の画面などは App のその tab で答える）。急ぐときだけ手で: hc send %s %q", addr, msg)
}

// Run は hc run --json --bg --outlive-parent --no-notify で template を起こす（contract.md §8 の起こし方）。
// --no-notify: 止まった・終わったは dispatcher が端末の土台から拾う。labels は "k=v" の並び。
func Run(name string, labels []string, cwd, template, msg string) (*Started, error) {
	args := []string{"run", "--json", "--bg", "--outlive-parent", "--no-notify", "--name", name}
	for _, l := range labels {
		args = append(args, "--label", l)
	}
	args = append(args, "--cwd", cwd, template, msg)
	out, err := Output(nil, "", "hc", args...)
	if err != nil {
		return nil, fmt.Errorf("hc run に失敗: %v", err)
	}
	var s Started
	if err := json.Unmarshal([]byte(out), &s); err != nil || s.Address == "" {
		return nil, fmt.Errorf("hc run の返事に address が無い: %s", out)
	}
	return &s, nil
}

// Worktree は NewWorktree が切ったもの。
type Worktree struct{ RepoDir, Path, Branch string }

// NewWorktree は origin の既定の branch の最新から、upstream の無い branch と worktree を切る。
func NewWorktree(repoDir, slug, id, title string) (*Worktree, error) {
	g := func(args ...string) (string, error) {
		return Output(nil, "", "git", append([]string{"-c", "url.https://github.com/.insteadOf=git@github.com:", "-C", repoDir}, args...)...)
	}
	sym, err := g("ls-remote", "--symref", "origin", "HEAD")
	def := ""
	for _, ln := range strings.Split(sym, "\n") {
		if f := strings.Fields(ln); len(f) == 3 && f[0] == "ref:" && f[2] == "HEAD" && strings.HasPrefix(f[1], "refs/heads/") {
			def = strings.TrimPrefix(f[1], "refs/heads/")
		}
	}
	if err != nil || def == "" {
		return nil, defaultBranchError(repoDir, err)
	}
	if _, err := g("fetch", "-q", "origin", "+refs/heads/"+def+":refs/remotes/origin/"+def); err != nil {
		return nil, fmt.Errorf("origin/%s を fetch できない（%s）", def, repoDir)
	}
	branch := slug + "/" + id
	if short := ShortTitle(title); short != "" {
		branch += "-" + short
	}
	if _, err := g("check-ref-format", "--branch", branch); err != nil {
		return nil, fmt.Errorf("branch 名が不正: %s", branch)
	}
	if _, err := g("show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		return nil, fmt.Errorf("branch %s がもうある", branch)
	}
	// origin にだけある同名の branch（clone が fetch していないもの）も見る。担当の push がぶつからないように
	if out, err := g("ls-remote", "--heads", "origin", "refs/heads/"+branch); err != nil {
		return nil, fmt.Errorf("origin の branch を確かめられない（%s）", repoDir)
	} else if out != "" {
		return nil, fmt.Errorf("branch %s が origin にもうある", branch)
	}
	path := WorktreePath(repoDir, slug, id)
	if _, err := os.Lstat(path); err == nil {
		return nil, fmt.Errorf("worktree の置き場がもうある: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if _, err := g("worktree", "add", "-q", "--no-track", "-b", branch, path, "refs/remotes/origin/"+def); err != nil {
		return nil, fmt.Errorf("worktree を作れない: %s", path)
	}
	return &Worktree{RepoDir: repoDir, Path: path, Branch: branch}, nil
}

// authFailure は git の stderr が認証の失敗らしいとき真（credential helper が未設定・token が無効・権限が無い）。
var authFailure = regexp.MustCompile(`(?i)could not read (username|password)|authentication failed|terminal prompts disabled|permission denied|invalid credentials|repository not found|returned error: 40[13]`)

// defaultBranchError は origin の既定の branch を引けなかった失敗の文。git の stderr（Output が err に入れた1行）を添え、
// 認証が原因らしいときは gh auth status・gh auth setup-git を案内する（セルで gh auth setup-git が未実行のとき、原因が文言に出ない）。
func defaultBranchError(repoDir string, cause error) error {
	msg := fmt.Sprintf("origin の既定の branch が分からない（%s）", repoDir)
	if cause != nil {
		msg += ": " + cause.Error()
	}
	if cause != nil && authFailure.MatchString(cause.Error()) {
		msg += "。認証が原因らしい: gh auth status でログインを確かめ、gh auth setup-git で git の資格情報を gh にする（boot が起動のたびに打つ。gh auth login のあと andon boot を打ち直してもよい）"
	}
	return errors.New(msg)
}

// WorktreePath は ~/.herdr/worktrees/<repo 名>/<slug>-<id>（andon issue finish が片付ける置き場と同じ）。
func WorktreePath(repoDir, slug, id string) string {
	return filepath.Join(os.Getenv("HOME"), ".herdr", "worktrees", filepath.Base(repoDir), slug+"-"+id)
}

// Remove は NewWorktree が作ったものを戻す（起こせなかったとき。やり直せるように）。
func (w *Worktree) Remove() {
	Output(nil, "", "git", "-C", w.RepoDir, "worktree", "remove", "--force", w.Path)
	Output(nil, "", "git", "-C", w.RepoDir, "branch", "-D", w.Branch)
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// ShortTitle は題の英字の短い形: ASCII の英数字以外を - にして小文字・30字まで（日本語だけの題なら空）。
func ShortTitle(title string) string {
	s := strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if len(s) > 30 {
		s = strings.TrimRight(s[:30], "-")
	}
	return s
}
