package pkgversion

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const c1 = "0123456789abcdef0123456789abcdef01234567"
const c2 = "89abcdef0123456789abcdef0123456789abcdef"

// pkg は /opt/andon に当たる偽の置き場を作る
func pkg(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"skills/worker/SKILL.md":    "# worker\n",
		"settings.json":             "{}\n",
		"schema/report.schema.json": "{}\n",
		"bin/andon":                 "#!/bin/sh\n",
	} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(root, "bin/andon"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func write(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func applied(t *testing.T, root, commit string) {
	t.Helper()
	write(t, filepath.Join(root, "applied.json"),
		`{"commit": "`+commit+`", "applied_at": "2026-11-19T00:00:00Z", "image_commit": "0123456789ab"}`+"\n")
}

func boolp(b *bool) string {
	if b == nil {
		return "null"
	}
	if *b {
		return "true"
	}
	return "false"
}

func check(t *testing.T, name string, got Info, commit any, source string, drift string) {
	t.Helper()
	gc := "null"
	if got.Commit != nil {
		gc = *got.Commit
	}
	wc := "null"
	if s, ok := commit.(string); ok {
		wc = s
	}
	if gc != wc || got.Source != source || boolp(got.Drift) != drift {
		t.Errorf("%s: commit=%s source=%s drift=%s reasons=%v（want %s %s %s）", name, gc, got.Source, boolp(got.Drift), got.Reasons, wc, source, drift)
	}
	if (got.Drift == nil || *got.Drift) && len(got.Reasons) == 0 {
		t.Errorf("%s: 食い違う・分からないのに理由が無い", name)
	}
}

func TestStampThenReadIsClean(t *testing.T) {
	root := pkg(t)
	if err := Stamp(root, c1); err != nil {
		t.Fatal(err)
	}
	check(t, "焼いた直後", Read(root), c1, "image", "false")
}

// ADR-0028 受け入れ条件 4 の土台: 置き場の1ファイルを書き換えると食い違う
func TestEditedFileIsDrift(t *testing.T) {
	root := pkg(t)
	if err := Stamp(root, c1); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "skills/worker/SKILL.md"), "# worker（手で書き換えた）\n")
	check(t, "1ファイルを書き換えた", Read(root), c1, "image", "true")
}

func TestAddedRemovedModeChangedIsDrift(t *testing.T) {
	cases := map[string]func(root string){
		"足した":        func(root string) { write(t, filepath.Join(root, "skills/new.md"), "x\n") },
		"消した":        func(root string) { os.Remove(filepath.Join(root, "settings.json")) },
		"実行の印":       func(root string) { os.Chmod(filepath.Join(root, "bin/andon"), 0o644) },
		"andon を替えた": func(root string) { write(t, filepath.Join(root, "bin/andon"), "#!/bin/sh\necho x\n") },
	}
	for name, edit := range cases {
		root := pkg(t)
		if err := Stamp(root, c1); err != nil {
			t.Fatal(err)
		}
		edit(root)
		check(t, name, Read(root), c1, "image", "true")
	}
}

// 記録のファイルと、python が書く cache は中身に数えない
func TestRecordsAndCacheAreNotContent(t *testing.T) {
	root := pkg(t)
	if err := Stamp(root, c1); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, "lib/andon/__pycache__"), 0o755)
	write(t, filepath.Join(root, "lib/andon/__pycache__/x.cpython-312.pyc"), "bytecode")
	check(t, "cache が増えた", Read(root), c1, "image", "false")
}

func TestDirtyRevision(t *testing.T) {
	root := pkg(t)
	if err := Stamp(root, c1+"-dirty"); err != nil {
		t.Fatal(err)
	}
	check(t, "汚れた checkout から焼いた", Read(root), c1, "image", "true")
}

func TestUnknownRevision(t *testing.T) {
	root := pkg(t)
	if err := Stamp(root, "unknown"); err != nil {
		t.Fatal(err)
	}
	got := Read(root)
	check(t, "元の commit が分からない", got, nil, "image", "true")
}

func TestBadRevisionRefused(t *testing.T) {
	for _, rev := range []string{"", "0123456789ab", strings.ToUpper(c1), c1 + "x"} {
		if err := Stamp(pkg(t), rev); err == nil {
			t.Errorf("%q を受けた", rev)
		}
	}
}

// 当て直したセル（ADR-0027）: applied.json の commit を読む
func TestAppliedWithStamp(t *testing.T) {
	root := pkg(t)
	applied(t, root, c2)
	if err := Stamp(root, c2); err != nil {
		t.Fatal(err)
	}
	check(t, "当て直して焼いた", Read(root), c2, "applied", "false")
}

// 前のバージョンの道具で当て直した（version.json が無い）: commit は読めるが、食い違いは確かめられない
func TestAppliedWithoutStamp(t *testing.T) {
	root := pkg(t)
	applied(t, root, c2)
	check(t, "印の無い当て直し", Read(root), c2, "applied", "null")
}

// 当て直しの記録と焼いた印が違う commit を指す: 手で入れ替えたので食い違い
func TestAppliedAndStampDisagree(t *testing.T) {
	root := pkg(t)
	if err := Stamp(root, c1); err != nil {
		t.Fatal(err)
	}
	applied(t, root, c2)
	check(t, "記録が食い違う", Read(root), c2, "applied", "true")
}

func TestNoRecord(t *testing.T) {
	check(t, "記録が無い", Read(pkg(t)), nil, "none", "null")
	check(t, "置き場が無い", Read(filepath.Join(t.TempDir(), "none")), nil, "none", "null")
}

func TestBrokenStamp(t *testing.T) {
	root := pkg(t)
	write(t, filepath.Join(root, "version.json"), "{")
	check(t, "壊れた印", Read(root), nil, "none", "null")
}

func TestMain_(t *testing.T) {
	root := pkg(t)
	t.Setenv("ANDON_HOME", root)
	var out, errb bytes.Buffer
	if rc := StampMain([]string{c1}, strings.NewReader(""), &out, &errb); rc != 0 {
		t.Fatalf("stamp: rc=%d err=%s", rc, errb.String())
	}
	out.Reset()
	if rc := Main(nil, strings.NewReader(""), &out, &errb); rc != 0 {
		t.Fatalf("read: rc=%d err=%s", rc, errb.String())
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("JSON でない: %v %s", err, out.String())
	}
	if got["commit"] != c1 || got["source"] != "image" || got["drift"] != false || got["root"] != root {
		t.Errorf("出力: %s", out.String())
	}
	errb.Reset()
	if rc := Main([]string{"bogus"}, strings.NewReader(""), &out, &errb); rc != 2 || !strings.HasPrefix(errb.String(), "usage: andon version") {
		t.Errorf("bogus: rc=%d err=%q", rc, errb.String())
	}
	for _, args := range [][]string{nil, {"unknown", "extra"}} {
		errb.Reset()
		if rc := StampMain(args, strings.NewReader(""), &out, &errb); rc != 2 || !strings.HasPrefix(errb.String(), "usage: andon version stamp") {
			t.Errorf("stamp %v: rc=%d err=%q", args, rc, errb.String())
		}
	}
}
