package digest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"andon/internal/flock"
	"andon/internal/waiting"
)

// Slack は検査の中に立てた偽の HTTP のサーバに向ける（ADR-0044 受け入れ条件 2・8〜11）。本物へは届かない:
// Env.SlackAPI は偽のサーバか空で、空なら送らずに誤りにする。

const (
	channel = "C0123456789"
	owner   = "owner@example.com"
)

// fakeToken は偽のトークン（そのままの形を repo に書かない）。
var fakeToken = "xo" + "xb-" + "test-0000-not-a-token"

type slackCall struct {
	path, auth, contentType string
	form                    map[string]string
	body                    []byte
}

type fakeSlack struct {
	srv *httptest.Server
	mu  sync.Mutex
	got []slackCall
	// completeError があれば files.completeUploadExternal がその誤りを返す
	completeError string
	// uploadStatus があれば中身を置く呼び出しがその status を返す
	uploadStatus int
	// before は応答の前に呼ばれる
	before func(path string)
}

func newFakeSlack(t *testing.T) *fakeSlack {
	t.Helper()
	s := &fakeSlack{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := slackCall{path: r.URL.Path, auth: r.Header.Get("Authorization"), contentType: r.Header.Get("Content-Type"), form: map[string]string{}}
		if c.contentType == "application/x-www-form-urlencoded" {
			if err := r.ParseForm(); err != nil {
				t.Errorf("form: %v", err)
			}
			for k := range r.PostForm {
				c.form[k] = r.PostForm.Get(k)
			}
		} else {
			c.body, _ = io.ReadAll(r.Body)
		}
		s.mu.Lock()
		s.got = append(s.got, c)
		before, status, completeError := s.before, s.uploadStatus, s.completeError
		s.mu.Unlock()
		if before != nil {
			before(c.path)
		}
		switch c.path {
		case "/api/files.getUploadURLExternal":
			fmt.Fprintf(w, `{"ok": true, "upload_url": "%s/upload/secret-slot", "file_id": "F0001"}`, s.srv.URL)
		case "/upload/secret-slot":
			if status != 0 {
				w.WriteHeader(status)
				return
			}
			fmt.Fprint(w, "OK")
		case "/api/files.completeUploadExternal":
			if completeError != "" {
				fmt.Fprintf(w, `{"ok": false, "error": "%s"}`, completeError)
				return
			}
			fmt.Fprint(w, `{"ok": true, "files": [{"id": "F0001"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *fakeSlack) set(f func(*fakeSlack)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(s)
}

func (s *fakeSlack) calls() []slackCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]slackCall{}, s.got...)
}

func projectWith(digest string) string {
	extra := ""
	if digest != "" {
		extra = `, "digest": ` + digest
	}
	return `{"version": 1, "project": {"title": "Sample", "prefix": "S"}, "repos": []` + extra + `}`
}

func toChannel(id string) string {
	return projectWith(`{"destinations": [{"kind": "slack", "channel": "` + id + `"}]}`)
}

// commitProject はセルの repo の project.json を author で commit する。body が空なら project.json の無い repo を作る。
func (f *fake) commitProject(body, author string) {
	f.t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		f.t.Skip("git が無い")
	}
	f.write(filepath.Join(f.ws, "andon.toml"), "owner = \""+owner+"\"\n")
	if body != "" {
		f.write(filepath.Join(f.ws, "projects", "sample-site", "project.json"), body)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "--allow-empty", "-m", "project"}} {
		cmd := exec.Command("git", append([]string{"-C", f.ws, "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=a", "GIT_AUTHOR_EMAIL="+author, "GIT_COMMITTER_NAME=a", "GIT_COMMITTER_EMAIL="+author)
		if out, err := cmd.CombinedOutput(); err != nil {
			f.t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

// withSlack は偽の Slack とトークンを Env に向ける。project.json は置かない。
func (f *fake) withSlack() *fakeSlack {
	f.t.Helper()
	s := newFakeSlack(f.t)
	f.env.SlackAPI = s.srv.URL + "/api/"
	f.env.Tokens = filepath.Join(filepath.Dir(f.root), "tokens")
	f.write(filepath.Join(f.env.Tokens, "slack.token"), fakeToken+"\n")
	return s
}

// ready は送り先のあるセルで、組み立てたところまで作る。
func ready(t *testing.T) (*fake, *fakeSlack) {
	t.Helper()
	f := newFake(t)
	s := f.withSlack()
	f.commitProject(toChannel(channel), owner)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.collect("S-0100")
	buildHTML(t, f, validText())
	return f, s
}

func (f *fake) sent() *Sent {
	f.t.Helper()
	s, err := readSent(filepath.Join(dir(f.root, "S-0100"), sentFile))
	if err != nil {
		f.t.Fatal(err)
	}
	return s
}

func slackRow(s *Sent) *Destination {
	for i := range s.Destinations {
		if s.Destinations[i].Kind == kindSlack {
			return &s.Destinations[i]
		}
	}
	return nil
}

// 受け入れ条件 9: 名前とバイト数、中身、チャンネルと短い文が、この順に 3 回来る。もう一度打つと来ない。
func TestSendSlack(t *testing.T) {
	t.Run("3回の呼び出し", func(t *testing.T) {
		f, s := ready(t)
		u, err := Send(f.env, "S-0100")
		if err != nil {
			t.Fatal(err)
		}
		got := s.calls()
		if len(got) != 3 {
			t.Fatalf("呼び出しが %d 回", len(got))
		}
		page, err := os.ReadFile(filepath.Join(dir(f.root, "S-0100"), htmlFile))
		if err != nil {
			t.Fatal(err)
		}
		name := "digest-sample-site-" + now.In(time.Local).Format("2006-01-02") + ".html"
		if c := got[0]; c.path != "/api/files.getUploadURLExternal" || c.form["filename"] != name || c.form["length"] != fmt.Sprint(len(page)) || c.auth != "Bearer "+fakeToken {
			t.Errorf("1回目: %s %v", c.path, c.form)
		}
		if c := got[1]; c.path != "/upload/secret-slot" || !bytes.Equal(c.body, page) || c.auth != "" || c.contentType != "application/octet-stream" {
			t.Errorf("2回目: %s %d バイト auth=%t", c.path, len(c.body), c.auth != "")
		}
		c := got[2]
		var files []map[string]string
		if err := json.Unmarshal([]byte(c.form["files"]), &files); err != nil || len(files) != 1 || files[0]["id"] != "F0001" || files[0]["title"] != name {
			t.Errorf("3回目の files: %q %v", c.form["files"], err)
		}
		if c.path != "/api/files.completeUploadExternal" || c.form["channel_id"] != channel || c.auth != "Bearer "+fakeToken {
			t.Errorf("3回目: %s %v", c.path, c.form)
		}
		for _, want := range []string{"ボードの表示が増えた。", "オーナーの待ちは 1 件。", "オーナーを待っているもの: 0 件", "マージした PR: 1 本（自動 1 本）", "全文: " + u} {
			if !strings.Contains(c.form["initial_comment"], want) {
				t.Errorf("短い文に %q が無い: %s", want, c.form["initial_comment"])
			}
		}
		if r := slackRow(f.sent()); r == nil || !r.OK || r.File != "F0001" || r.Channel != channel || r.Error != "" {
			t.Errorf("sent.json: %+v", r)
		}
		if again, err := Send(f.env, "S-0100"); err != nil || again != u || len(s.calls()) != 3 {
			t.Errorf("打ち直しで送った: %v・%d 回", err, len(s.calls()))
		}
		// 送ったあとに送り先を変えても、同じ 1 枚は送り直さない
		f.commitProject(toChannel("C9999999999"), owner)
		if _, err := Send(f.env, "S-0100"); err != nil || len(s.calls()) != 3 {
			t.Errorf("送った 1 枚を別のチャンネルへ送った: %v・%d 回", err, len(s.calls()))
		}
	})

	t.Run("待ちが40件", func(t *testing.T) {
		f := newFake(t)
		s := f.withSlack()
		f.commitProject(toChannel(channel), owner)
		f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
		for i := range 40 {
			f.mergeWait(fmt.Sprintf("S-%04d", 200+i), 200+i, sha2, strings.Repeat("長", 700))
		}
		f.collect("S-0100")
		buildHTML(t, f, validText())
		u, err := Send(f.env, "S-0100")
		if err != nil {
			t.Fatal(err)
		}
		got := s.calls()
		if len(got) != 3 {
			t.Fatalf("呼び出しが %d 回", len(got))
		}
		msg := got[2].form["initial_comment"]
		shown := strings.Count(msg, "\n- ")
		if n := utf8.RuneCountInString(msg); n > messageMax || shown >= messageTop {
			t.Errorf("%d 字・待ちの行 %d", n, shown)
		}
		for _, want := range []string{"オーナーを待っているもの: 40 件", fmt.Sprintf("（ほか %d 件は全文に載せた）", 40-shown), "全文: " + u} {
			if !strings.Contains(msg, want) {
				t.Errorf("短い文に %q が無い", want)
			}
		}
	})

	t.Run("同時に2本打っても1度", func(t *testing.T) {
		f, s := ready(t)
		s.set(func(s *fakeSlack) { s.before = func(string) { time.Sleep(50 * time.Millisecond) } })
		var wg sync.WaitGroup
		for range 2 {
			wg.Go(func() {
				if _, err := Send(f.env, "S-0100"); err != nil {
					t.Errorf("send: %v", err)
				}
			})
		}
		wg.Wait()
		if n := len(s.calls()); n != 3 {
			t.Errorf("呼び出しが %d 回", n)
		}
	})

	t.Run("組み立てたものと違う HTML は送らない", func(t *testing.T) {
		f, s := ready(t)
		p := filepath.Join(dir(f.root, "S-0100"), htmlFile)
		page, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f.write(p, string(page)+"<!-- 足した中身 -->")
		if _, err := Send(f.env, "S-0100"); err == nil || !strings.Contains(err.Error(), "組み立てたものと違う") {
			t.Errorf("断らない: %v", err)
		}
		if len(s.calls()) != 0 {
			t.Error("送った")
		}
	})

	t.Run("伏せた秘密のある 1 枚も組み立てたものと同じに読む", func(t *testing.T) {
		f, s := ready(t)
		secret := "password=abc[def] bearer " + strings.Repeat("a", 24) + " " + fakeToken
		buildHTML(t, f, strings.Replace(validText(), "ボードの表示が増えた。", "ボードの表示が増えた。"+secret, 1))
		if _, err := Send(f.env, "S-0100"); err != nil {
			t.Fatalf("伏せた 1 枚を送れない: %v", err)
		}
		if got := s.calls(); len(got) != 3 || bytes.Contains(got[1].body, []byte(fakeToken)) || strings.Contains(got[2].form["initial_comment"], fakeToken) {
			t.Error("伏せた秘密が外へ出た")
		}
	})

	t.Run("短い文は message.txt でなく JSON から作る", func(t *testing.T) {
		f, s := ready(t)
		f.write(filepath.Join(dir(f.root, "S-0100"), messageFile), "差し替えた文")
		if _, err := Send(f.env, "S-0100"); err != nil {
			t.Fatal(err)
		}
		if got := s.calls(); len(got) != 3 || strings.Contains(got[2].form["initial_comment"], "差し替えた文") {
			t.Error("message.txt の文を送った")
		}
	})

	t.Run("送っている間は済んでいないと書いてある", func(t *testing.T) {
		f, s := ready(t)
		seen := 0
		s.set(func(s *fakeSlack) {
			s.before = func(string) {
				seen++
				sent, err := readSent(filepath.Join(dir(f.root, "S-0100"), sentFile))
				if err != nil {
					t.Errorf("送る前に sent.json が無い: %v", err)
					return
				}
				if _, done := sent.done(); done {
					t.Error("送っている途中なのに済んだと読める（ここで落ちると届かない期間ができる）")
				}
			}
		})
		if _, err := Send(f.env, "S-0100"); err != nil || seen != 3 {
			t.Errorf("%v・%d 回", err, seen)
		}
	})

	t.Run("前のバージョンの sent.json", func(t *testing.T) {
		f, s := ready(t)
		in, err := readInput(f.root, "S-0100")
		if err != nil {
			t.Fatal(err)
		}
		u := filesURL(f.env, in)
		f.lib("S-0100", "digest/sent.json", `{"issue": "S-0100", "since": "`+in.Since+`", "until": "`+in.Until+`", "destinations": [{"kind": "files", "at": "`+in.Until+`", "ok": true, "url": "`+u+`"}]}`)
		if got, err := Send(f.env, "S-0100"); err != nil || got != u || len(s.calls()) != 3 {
			t.Errorf("files だけ済んだ記録から Slack へ送らない: %v %s", err, got)
		}
	})
}

// published は files/ に出した HTML。
func (f *fake) published() []byte {
	f.t.Helper()
	names, err := filepath.Glob(filepath.Join(f.ws, "files", "*-S-0100.html"))
	if err != nil || len(names) != 1 {
		f.t.Fatalf("files/ の HTML: %v %v", names, err)
	}
	b, err := os.ReadFile(names[0])
	if err != nil {
		f.t.Fatal(err)
	}
	return b
}

type writerFunc func(p []byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) { return w(p) }

// 反論で見つけた疑い: files/ と Slack と sent.json が、同じ 1 枚を指す。
func TestSendSameDigest(t *testing.T) {
	t.Run("送り直す前に組み立て直した", func(t *testing.T) {
		f, s := ready(t)
		s.set(func(s *fakeSlack) { s.completeError = "not_in_channel" })
		if _, err := Send(f.env, "S-0100"); err == nil {
			t.Fatal("送れた")
		}
		buildHTML(t, f, strings.Replace(validText(), "ボードの表示が増えた。", "ボードの表示を直した。", 1))
		s.set(func(s *fakeSlack) { s.completeError = "" })
		if _, err := Send(f.env, "S-0100"); err != nil {
			t.Fatal(err)
		}
		if got := s.calls(); !bytes.Equal(got[len(got)-2].body, f.published()) {
			t.Error("files/ と Slack の中身が違う（短い文の URL が古い 1 枚を指す）")
		}
	})

	t.Run("鍵を待つ間に集め直して組み立て直した", func(t *testing.T) {
		f, s := ready(t)
		unlock, err := flock.Lock(filepath.Join(dir(f.root, "S-0100"), sentFile+".lock"))
		if err != nil {
			t.Fatal(err)
		}
		env, done := f.env, make(chan error, 1)
		go func() {
			_, err := Send(env, "S-0100")
			done <- err
		}()
		time.Sleep(300 * time.Millisecond) // send が鍵の待ちに入る
		f.env.Now = now.Add(time.Hour)
		in := f.collect("S-0100")
		buildHTML(t, f, validText())
		page, err := os.ReadFile(filepath.Join(dir(f.root, "S-0100"), htmlFile))
		unlock()
		if err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if got := s.calls(); len(got) != 3 || !bytes.Equal(got[1].body, page) || f.sent().Until != in.Until {
			t.Error("鍵を待つ前に読んだ古い 1 枚を送った")
		}
	})

	t.Run("照らしたあとに HTML が変わった", func(t *testing.T) {
		f, _ := ready(t)
		p := filepath.Join(dir(f.root, "S-0100"), htmlFile)
		page, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f.commitProject(toChannel("C9999999999"), "worker@example.com")
		// 送り先を読めない知らせは、照らしたあと・files/ に出す前に出る
		f.env.Stderr = writerFunc(func(b []byte) (int, error) { return len(b), os.WriteFile(p, []byte("別の中身"), 0o600) })
		if _, err := Send(f.env, "S-0100"); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(f.published(), page) {
			t.Error("照らしていない中身を files/ に出した")
		}
	})
}

// 受け入れ条件 10: not_in_channel は終了コード 1 と招く手順の 1 行。sent.json に理由。直して打ち直すと送る。
func TestSendFails(t *testing.T) {
	for _, code := range []string{"not_in_channel", "channel_not_found"} {
		t.Run(code, func(t *testing.T) {
			f, s := ready(t)
			s.set(func(s *fakeSlack) { s.completeError = code })
			var out, errb bytes.Buffer
			rc := run([]string{"send", "S-0100"}, nil, &out, &errb, func() (Env, error) { return f.env, nil })
			if rc != 1 || strings.Count(errb.String(), "\n") != 1 || !strings.Contains(errb.String(), "bot をチャンネル "+channel+" に招いて") || !strings.Contains(out.String(), "http://files.cell.localhost:") {
				t.Fatalf("rc=%d out=%q err=%q", rc, out.String(), errb.String())
			}
			if r := slackRow(f.sent()); r == nil || r.OK || !strings.Contains(r.Error, code) {
				t.Fatalf("理由が残らない: %+v", r)
			}
			s.set(func(s *fakeSlack) { s.completeError = "" })
			out.Reset()
			errb.Reset()
			if rc := run([]string{"send", "S-0100"}, nil, &out, &errb, func() (Env, error) { return f.env, nil }); rc != 0 {
				t.Fatalf("送り直せない: %q", errb.String())
			}
			if r := slackRow(f.sent()); !r.OK || r.File != "F0001" || len(s.calls()) != 6 {
				t.Errorf("送り直しの記録: %+v", r)
			}
		})
	}

	t.Run("中身を置けなければチャンネルへ出さない", func(t *testing.T) {
		f, s := ready(t)
		s.set(func(s *fakeSlack) { s.uploadStatus = http.StatusInternalServerError })
		_, err := Send(f.env, "S-0100")
		if err == nil || !strings.Contains(err.Error(), "HTTP 500") || strings.Contains(err.Error(), "secret-slot") || len(s.calls()) != 2 {
			t.Errorf("%v・%d 回", err, len(s.calls()))
		}
	})

	t.Run("届かないときに置き場の URL を出さない", func(t *testing.T) {
		f, _ := ready(t)
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"ok": true, "upload_url": "http://127.0.0.1:1/secret-slot", "file_id": "F0001"}`)
		}))
		defer api.Close()
		f.env.SlackAPI = api.URL + "/api/"
		if _, err := Send(f.env, "S-0100"); err == nil || strings.Contains(err.Error(), "secret-slot") {
			t.Errorf("%v", err)
		}
		if r := slackRow(f.sent()); r == nil || r.OK || strings.Contains(r.Error, "secret-slot") {
			t.Errorf("%+v", r)
		}
	})

	t.Run("応答に混ざったトークンを写さない", func(t *testing.T) {
		f, s := ready(t)
		s.set(func(s *fakeSlack) { s.completeError = fakeToken })
		if _, err := Send(f.env, "S-0100"); err == nil || strings.Contains(err.Error(), fakeToken) || strings.Contains(slackRow(f.sent()).Error, fakeToken) {
			t.Errorf("%v", err)
		}
	})

	t.Run("トークンが無い", func(t *testing.T) {
		f, s := ready(t)
		f.env.Tokens = t.TempDir()
		if _, err := Send(f.env, "S-0100"); err == nil || !strings.Contains(err.Error(), "andon digest auth slack") || len(s.calls()) != 0 {
			t.Errorf("%v", err)
		}
	})

	t.Run("失敗のあとに送り先を変えたら新しい先へ", func(t *testing.T) {
		f, s := ready(t)
		s.set(func(s *fakeSlack) { s.completeError = "not_in_channel" })
		if _, err := Send(f.env, "S-0100"); err == nil {
			t.Fatal("送れた")
		}
		s.set(func(s *fakeSlack) { s.completeError = "" })
		f.commitProject(toChannel("C9999999999"), owner)
		if _, err := Send(f.env, "S-0100"); err != nil {
			t.Fatal(err)
		}
		if r := slackRow(f.sent()); !r.OK || r.Channel != "C9999999999" {
			t.Errorf("%+v", r)
		}
	})

	t.Run("失敗のあとに送り先を消したら files だけで済む", func(t *testing.T) {
		f, s := ready(t)
		s.set(func(s *fakeSlack) { s.completeError = "not_in_channel" })
		if _, err := Send(f.env, "S-0100"); err == nil {
			t.Fatal("送れた")
		}
		f.commitProject(projectWith(""), owner)
		if _, err := Send(f.env, "S-0100"); err != nil {
			t.Fatal(err)
		}
		if _, done := f.sent().done(); !done || slackRow(f.sent()) != nil {
			t.Errorf("%+v", f.sent())
		}
	})

	t.Run("失敗のあとに project.json を信じられない", func(t *testing.T) {
		f, s := ready(t)
		s.set(func(s *fakeSlack) { s.completeError = "not_in_channel" })
		if _, err := Send(f.env, "S-0100"); err == nil {
			t.Fatal("送れた")
		}
		f.commitProject(toChannel("C9999999999"), "worker@example.com")
		if _, err := Send(f.env, "S-0100"); err == nil || len(s.calls()) != 3 {
			t.Errorf("%v・%d 回", err, len(s.calls()))
		}
		if r := slackRow(f.sent()); r == nil || r.OK {
			t.Errorf("届いていない期間を送ったことにした: %+v", r)
		}
	})

	// HEAD に無いことと、読めないことは違う（読めないのに行を落とすと、届かない期間が送ったことになる）
	for name, breakRepo := range map[string]func(f *fake) error{
		"git が repo を読めない": func(f *fake) error { return os.Rename(filepath.Join(f.ws, ".git"), filepath.Join(f.ws, "git.away")) },
		"HEAD が壊れている": func(f *fake) error {
			return os.WriteFile(filepath.Join(f.ws, ".git", "HEAD"), []byte("broken\n"), 0o600)
		},
	} {
		t.Run("失敗のあとに"+name, func(t *testing.T) {
			f, s := ready(t)
			s.set(func(s *fakeSlack) { s.completeError = "not_in_channel" })
			if _, err := Send(f.env, "S-0100"); err == nil {
				t.Fatal("送れた")
			}
			if err := breakRepo(f); err != nil {
				t.Fatal(err)
			}
			if _, err := Send(f.env, "S-0100"); err == nil {
				t.Error("終了コード 0")
			}
			if r := slackRow(f.sent()); r == nil || r.OK {
				t.Error("届いていない期間を送ったことにした")
			}
			if err := CheckDestinations(f.env, io.Discard); err == nil || !strings.Contains(err.Error(), "読めない") {
				t.Errorf("check の理由: %v", err)
			}
		})
	}
}

// 受け入れ条件 2 の Slack の部分: Slack へ送れなかった前のダイジェストは期間に数えない。送り直せば数える。
func TestPeriodSlack(t *testing.T) {
	f, s := ready(t)
	s.set(func(s *fakeSlack) { s.completeError = "not_in_channel" })
	if _, err := Send(f.env, "S-0100"); err == nil {
		t.Fatal("送れた")
	}
	f.env.Now = now.Add(2 * time.Hour)
	f.issue("S-0101", "doing", "routine: daily-digest", "profile: claude")
	if in := f.collect("S-0101"); in.Previous != nil || in.Since != "2026-11-20T12:00:00Z" {
		t.Errorf("Slack へ送れなかったものを数えた: since=%s previous=%+v", in.Since, in.Previous)
	}
	s.set(func(s *fakeSlack) { s.completeError = "" })
	if _, err := Send(f.env, "S-0100"); err != nil {
		t.Fatal(err)
	}
	if in := f.collect("S-0101"); in.Previous == nil || in.Previous.Issue != "S-0100" || in.Since != "2026-11-21T10:00:00Z" {
		t.Errorf("送り直したものを数えない: since=%s previous=%+v", in.Since, in.Previous)
	}
}

// 受け入れ条件 11: check は送るときと同じ 3 回。auth は mode 600。どの出力にも記録にもトークンが無い。
func TestCheck(t *testing.T) {
	t.Run("3回の呼び出し", func(t *testing.T) {
		f, s := ready(t)
		var out, errb bytes.Buffer
		if rc := run([]string{"check"}, nil, &out, &errb, func() (Env, error) { return f.env, nil }); rc != 0 {
			t.Fatalf("rc=%d err=%q", rc, errb.String())
		}
		got := s.calls()
		if len(got) != 3 {
			t.Fatalf("呼び出しが %d 回", len(got))
		}
		for i, path := range []string{"/api/files.getUploadURLExternal", "/upload/secret-slot", "/api/files.completeUploadExternal"} {
			if got[i].path != path {
				t.Errorf("%d 回目が %s", i+1, got[i].path)
			}
		}
		if got[0].form["filename"] != "digest-check-sample-site.txt" || len(got[1].body) == 0 || len(got[1].body) > 1024 || got[2].form["channel_id"] != channel {
			t.Errorf("確かめのファイル: %v %v", got[0].form, got[2].form)
		}
		if !strings.Contains(out.String(), channel) {
			t.Errorf("out=%q", out.String())
		}
		if _, err := os.Stat(filepath.Join(dir(f.root, "S-0100"), sentFile)); err == nil {
			t.Error("check が sent.json を書いた")
		}
	})

	for name, c := range map[string]struct{ body, author, why string }{
		"project.json が無い": {"", owner, "設定が無い"},
		"digest が無い":       {projectWith(""), owner, "設定が無い"},
		"空":                {projectWith(`{"destinations": []}`), owner, "設定が無い"},
		"信じられない":           {toChannel(channel), "worker@example.com", "読めない"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFake(t)
			s := f.withSlack()
			f.commitProject(c.body, c.author)
			var out, errb bytes.Buffer
			rc := run([]string{"check"}, nil, &out, &errb, func() (Env, error) { return f.env, nil })
			if rc != 1 || out.Len() != 0 || strings.Count(errb.String(), "\n") != 1 || !strings.Contains(errb.String(), c.why) || len(s.calls()) != 0 {
				t.Errorf("rc=%d out=%q err=%q %d 回", rc, out.String(), errb.String(), len(s.calls()))
			}
		})
	}

	t.Run("トークンはどこにも出ない", func(t *testing.T) {
		for _, completeError := range []string{"", "invalid_auth"} {
			f, s := ready(t)
			s.set(func(s *fakeSlack) { s.completeError = completeError })
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(filepath.Dir(f.root), "config"))
			f.env.Tokens = TokenDir()
			env := func() (Env, error) { return f.env, nil }
			var out bytes.Buffer // 標準出力と標準エラーをまとめて見る
			if rc := run([]string{"auth", "slack"}, strings.NewReader(fakeToken+"\n"), &out, &out, env); rc != 0 {
				t.Fatalf("auth: %q", out.String())
			}
			if st, err := os.Stat(filepath.Join(f.env.Tokens, "slack.token")); err != nil || st.Mode().Perm() != 0o600 {
				t.Fatalf("mode: %v %v", st, err)
			}
			run([]string{"auth", fakeToken}, strings.NewReader(""), &out, &out, env) // 引数に貼り間違えても出さない
			run([]string{"collect", "S-0100"}, nil, &out, &out, env)
			run([]string{"build", "S-0100", "-"}, strings.NewReader(validText()), &out, &out, env)
			run([]string{"check"}, nil, &out, &out, env)
			run([]string{"send", "S-0100"}, nil, &out, &out, env)
			if len(s.calls()) != 6 {
				t.Fatalf("check と send が Slack を呼んでいない: %q", out.String())
			}
			if strings.Contains(out.String(), fakeToken) {
				t.Errorf("出力にトークン: %q", out.String())
			}
			files := 0
			for _, root := range []string{f.root, filepath.Join(f.ws, "files")} {
				err := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
					if err != nil || e.IsDir() {
						return err
					}
					b, err := os.ReadFile(p)
					files++
					if bytes.Contains(b, []byte(fakeToken)) {
						t.Errorf("%s にトークン", p)
					}
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if files == 0 {
				t.Fatal("見たファイルが無い")
			}
		}
	})
}

// 短い文: 行が無くても長すぎるときに URL と削った数を残す。タイトルの <!channel> で全員を呼ばない。
func TestMessageKeepsURLAndNoMention(t *testing.T) {
	const u = "http://files.cell.localhost:1355/x.html"
	in := &Input{Title: "デイリーダイジェスト", Project: "sample-site", Since: "2026-11-20T10:00:00Z", Until: "2026-11-21T10:00:00Z"}
	in.Waiting.Items = []waiting.Item{{Kind: "merge", ID: "S-0001", Title: "直す"}}
	msg := Message(&Text{Headline: []string{strings.Repeat("長", messageMax), "二。"}}, in, newLoc(""), u)
	if utf8.RuneCountInString(msg) > messageMax || !strings.Contains(msg, "全文: "+u) || !strings.Contains(msg, "ほか 1 件") || !strings.Contains(msg, "オーナーを待っているもの: 1 件") {
		t.Errorf("URL か数が落ちた: %d 字", utf8.RuneCountInString(msg))
	}
	empty := *in
	empty.Waiting.Items = nil
	if msg := Message(&Text{Headline: []string{strings.Repeat("長", messageMax), "二。"}}, &empty, newLoc(""), u); utf8.RuneCountInString(msg) > messageMax || !strings.Contains(msg, "オーナーを待っているもの: 0 件") {
		t.Errorf("待ちが無いときに数の行が落ちた: %d 字", utf8.RuneCountInString(msg))
	}
	in.Waiting.Items[0].Title = "<!channel> を呼ぶ"
	msg = Message(&Text{Headline: []string{"<https://example.com|確認>", "二。"}}, in, newLoc(""), u)
	if strings.ContainsAny(msg, "<>") || !strings.Contains(msg, "＜!channel＞") {
		t.Errorf("Slack が解釈する形が残った: %s", msg)
	}
}
