package spawn

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShortTitle(t *testing.T) {
	for in, want := range map[string]string{"課金の設計": "", "Billing の設計": "billing", "A very long English title for the design doc": "a-very-long-english-title-for"} {
		if got := ShortTitle(in); got != want {
			t.Errorf("ShortTitle(%q)=%q want %q", in, got, want)
		}
	}
}

func TestDefaultBranchErrorAuthHint(t *testing.T) {
	auth := fmt.Errorf("exit status 128: fatal: could not read Username for 'https://github.com': No such device or address")
	got := defaultBranchError("/repo", auth).Error()
	for _, want := range []string{"/repo", "could not read Username", "gh auth status", "gh auth setup-git"} {
		if !strings.Contains(got, want) {
			t.Errorf("認証の失敗の文言に %q が無い: %s", want, got)
		}
	}
	other := defaultBranchError("/repo", fmt.Errorf("exit status 128: fatal: unable to access: Could not resolve host")).Error()
	if strings.Contains(other, "gh auth") || !strings.Contains(other, "Could not resolve host") {
		t.Errorf("認証でない失敗に案内を出さず、git の stderr は添える: %s", other)
	}
	if got := defaultBranchError("/repo", nil).Error(); strings.Contains(got, "gh auth") {
		t.Errorf("原因が無いときに案内を出さない: %s", got)

	}
}

// 最初のメッセージは、送れて動き出さなかった（started=false）のでもないときだけ届いたとみる。
// started が無い（状態を報告しない program）は誰にも分からないので届いたとみる。
func TestFirstDelivered(t *testing.T) {
	yes, no := true, false
	for _, c := range []struct {
		name string
		f    First
		want bool
	}{
		{"送れなかった", First{Sent: false, Reason: "t9 is still starting: its output has not settled"}, false},
		{"送ったが動き出さない", First{Sent: true, Started: &no}, false},
		{"送って動き出した", First{Sent: true, Started: &yes}, true},
		{"状態を報告しない", First{Sent: true}, true},
	} {
		if got := c.f.Delivered(); got != c.want {
			t.Errorf("%s: Delivered()=%v", c.name, got)
		}
	}
	s := Started{First: First{Sent: true, Started: &no}}
	if r := s.Reason(); r != "送ったが動き出さなかった（起動の途中で入力を捨てられたとみる）" {
		t.Errorf("Reason()=%q", r)
	}
}

func TestNewSession(t *testing.T) {
	a, b := NewSession(), NewSession()
	if !SessionRe.MatchString(a) || a == b || a[14] != '4' {
		t.Errorf("NewSession: %q %q", a, b)
	}
}

func TestGuide(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("ANDON_WORKSPACE", ws)
	if g := Guide(); g != "" {
		t.Errorf("ファイルが無いのに入った: %q", g)
	}
	if err := os.WriteFile(filepath.Join(ws, GuideFile), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := "\nオーナーの指針 `" + filepath.Join(ws, GuideFile) + "` も読む。"
	if g := Guide(); g != want {
		t.Errorf("got %q want %q", g, want)
	}
}

// 案件の repo の置き場は GHQ_ROOT、無ければ ghq root。どちらも無ければ誤り。
func TestGhqRoot(t *testing.T) {
	t.Setenv("GHQ_ROOT", "/w/.ghq")
	if got, err := GhqRoot(); err != nil || got != "/w/.ghq" {
		t.Errorf("GHQ_ROOT: %q %v", got, err)
	}
	if got, err := RepoPath("github.com/example/app"); err != nil || got != filepath.Join("/w/.ghq", "github.com/example/app") {
		t.Errorf("RepoPath: %q %v", got, err)
	}
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "ghq"), []byte("#!/bin/sh\necho /from/ghq\n"), 0o755)
	t.Setenv("PATH", bin)
	t.Setenv("GHQ_ROOT", "")
	if got, err := GhqRoot(); err != nil || got != "/from/ghq" {
		t.Errorf("ghq root: %q %v", got, err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := GhqRoot(); err == nil {
		t.Error("ghq が無いのに誤りにならない")
	}
}

// WithBody は hc run が届ける最初のメッセージ（template の本文、空行、課題の文）を組む。hc send で送り直す文はこれにする。
func TestWithBody(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := TemplatesDir()
	if dir != filepath.Join(home, ".config", "herdr-core", "templates") {
		t.Fatalf("TemplatesDir=%q", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"worker": "---\nname: worker\ndescription: d\ncommand: claude\n---\n役目の1行目。\n役目の2行目。\n",
		"empty":  "---\nname: empty\ndescription: d\ncommand: claude\n---\n\n",
		"bare":   "本文だけ。\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	msg := "課題 `sample-site/S-1`。\n依頼書は `/x/brief.md`。"
	for tpl, want := range map[string]string{
		"worker":  "役目の1行目。\n役目の2行目。\n\n" + msg,
		"bare":    "本文だけ。\n\n" + msg,
		"empty":   msg, // 本文が空: hc run も課題の文だけを届ける
		"missing": msg, // 読めない: 今までどおり課題の文だけ
		"":        msg,
	} {
		if got := WithBody(tpl, msg); got != want {
			t.Errorf("WithBody(%q)=%q want %q", tpl, got, want)
		}
	}
	// 手で送る案内の行も本文つきで、案内の最後に置く（ラベルを付けられないほうの案内。hc が無いので MarkUnsent は落ちる）
	t.Setenv("PATH", t.TempDir())
	if got := ResendHint("t42", "worker", msg); !strings.HasSuffix(got, "手で送る: "+SendCommand("t42", "worker", msg)) {
		t.Errorf("ResendHint=%q", got)
	}
}

// SendCommand の行を bash にそのまま貼ると、hc send は hc run が届ける文（WithBody）をそのまま受け取る。
// 本文の `…`・$・単引用符を展開させず、改行は改行のまま渡す。
func TestSendCommandInBash(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash が無い")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(TemplatesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: codex-review\ndescription: d\ncommand: codex\n---\n`/opt/andon/skills/worker/SKILL.md` を最初に読む。\n$HOME も 'x' も $(echo no) もそのまま。\n"
	if err := os.WriteFile(filepath.Join(TemplatesDir(), "codex-review.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// 偽の hc: 受け取った引数を1つずつ NUL で区切って書く
	bin := t.TempDir()
	got := filepath.Join(bin, "args")
	if err := os.WriteFile(filepath.Join(bin, "hc"), []byte("#!/usr/bin/env bash\nprintf '%s\\0' \"$@\" >"+got+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	msg := "課題 `sample-site/S-1`。\n依頼書は `/x/brief.md`。"
	line := SendCommand("t42", "codex-review", msg)
	cmd := exec.Command(bash, "-c", line)
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bash -c %q: %v %s", line, err, out)
	}
	b, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"send", "t42", WithBody("codex-review", msg)}
	if args := strings.Split(strings.TrimSuffix(string(b), "\x00"), "\x00"); len(args) != 3 || args[0] != want[0] || args[1] != want[1] || args[2] != want[2] {
		t.Errorf("hc が受け取った引数=%q want %q", args, want)
	}
	if !strings.HasPrefix(want[2], "`/opt/andon/skills/worker/SKILL.md` を最初に読む。\n$HOME も 'x' も $(echo no) もそのまま。\n\n課題 ") {
		t.Errorf("本文が足されていない: %q", want[2])
	}
}

// 機械の送り手の印: 先頭に角括弧の名前を1つ置くだけで、文の中身は変えない。照らすのは dispatcher と gate の印だけ。
func TestMark(t *testing.T) {
	if got := Mark(FromGate, "S-7（タイトル）: gate が fix を返した。"); got != "[gate] S-7（タイトル）: gate が fix を返した。" {
		t.Errorf("Mark: %q", got)
	}
	for s, want := range map[string]bool{
		Mark(FromDispatcher, "続けて"): true,
		Mark(FromGate, "直して"):       true,
		"\n [gate] 直して":             true,
		"[dispatcher]続けて":           false,
		"[cos] オーナーの文":              false,
		"オーナーの文 [gate] ":            false,
		"":                          false,
	} {
		if got := Marked(s); got != want {
			t.Errorf("Marked(%q) = %v", s, got)
		}
	}
}
