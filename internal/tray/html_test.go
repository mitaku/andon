package tray

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, dir, rel, body string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// 受け入れ条件 9（Go の側）: 相対で引いた画像とスタイルを埋め、1枚で表示できる写しにする。# のリンクは同じ枠、外へのリンクは新しいタブ。
func TestEmbed(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "img/a.png", "PNG")
	put(t, dir, "css/s.css", `body{background:url('../img/a.png')} @font-face{src:url(f.woff2)}`)
	put(t, dir, "css/f.woff2", "FONT")
	put(t, dir, "js/app.js", `document.title = "x</script>";`)
	file := put(t, dir, "index.html", `<!doctype html><html><head><title>報告</title>
<link rel="stylesheet" href="css/s.css"><link rel="icon" href="img/a.png">
<link rel="stylesheet" href="https://cdn.example.com/x.css"></head>
<body><img src="img/a.png?v=1"><img src="data:image/png;base64,AA=="><img src="missing.png"><img src="/root.png">
<a href="#sec">目次</a><a href="https://example.com/">外</a><div style="background:url(img/a.png)"></div>
<script src="js/app.js"></script><h2 id="sec">節</h2></body></html>`)
	body, _ := os.ReadFile(file)
	e, err := Embed(file, body)
	if err != nil {
		t.Fatal(err)
	}
	out := string(e.Body)
	png := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("PNG"))
	font := "data:font/woff2;base64," + base64.StdEncoding.EncodeToString([]byte("FONT"))
	for _, want := range []string{
		`<base target="_blank"/>`,
		`<style>body{background:url("` + png + `")} @font-face{src:url("` + font + `")}</style>`,
		`<link rel="icon" href="` + png + `"/>`,
		`<img src="` + png + `"/>`,
		`<img src="data:image/png;base64,AA=="/>`,
		`<a href="#sec" target="_self">目次</a>`,
		`<a href="https://example.com/">外</a>`,
		`style="background:url(&#34;` + png + `&#34;)"`,
		`<script>document.title = "x<\/script>";</script>`,
		`<link rel="stylesheet" href="https://cdn.example.com/x.css"/>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("写しに %s が無い:\n%s", want, out)
		}
	}
	if strings.Contains(out, `src="js/app.js"`) || strings.Contains(out, `href="css/s.css"`) {
		t.Errorf("埋めた参照が残った:\n%s", out)
	}
	if e.Left != 1 {
		t.Errorf("残した外の参照 = %d（https のスタイルの1つ。リンクは数えない）", e.Left)
	}
	if strings.Join(e.Missing, ",") != "missing.png,/root.png" {
		t.Errorf("読めなかった参照 = %v", e.Missing)
	}
}

func TestEmbedKeepsExistingBase(t *testing.T) {
	file := put(t, t.TempDir(), "a.html", `<html><head><base href="x/"></head><body></body></html>`)
	body, _ := os.ReadFile(file)
	e, err := Embed(file, body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(e.Body), `<base href="x/" target="_blank"/>`) || strings.Count(string(e.Body), "<base") != 1 {
		t.Errorf("base: %s", e.Body)
	}
}

func TestTitles(t *testing.T) {
	if got := HTMLTitle([]byte("<html><head><title>\n 週の\n報告 </title></head></html>")); got != "週の 報告" {
		t.Errorf("HTML: %q", got)
	}
	if got := MarkdownTitle([]byte("前書き\n```\n# 違う\n```\n## 本当の見出し ##\n# 次\n")); got != "本当の見出し" {
		t.Errorf("Markdown: %q", got)
	}
	if got := MarkdownTitle([]byte("見出しなし\n")); got != "" {
		t.Errorf("見出しなし: %q", got)
	}
}

func TestContentType(t *testing.T) {
	for p, want := range map[string]string{
		"a.HTML": "text/html; charset=utf-8", "a.md": "text/markdown; charset=utf-8", "a.png": "image/png",
		"a.pdf": "application/pdf", "a.zip": "application/octet-stream", "a": "application/octet-stream",
	} {
		if got := ContentType(p); got != want {
			t.Errorf("%s: %q", p, got)
		}
	}
}

// HTML のコメント（CDATA を含む。tokenizer はコメントとして返す）は画面に出ないので、秘密が残っても誰も気づけない。伏せる
func TestRedactHTMLComments(t *testing.T) {
	for name, in := range map[string]string{
		"コメント":  "<p>x</p><!-- token: " + fakeSecret + " -->",
		"CDATA": "<svg><![CDATA[token: " + fakeSecret + "]]></svg>",
	} {
		if got := string(RedactHTML([]byte(in))); strings.Contains(got, fakeSecret) {
			t.Errorf("%s: 秘密がそのまま残る: %s", name, got)
		}
	}
	// 当たらないコメントは元のバイトのまま
	in := "<p>x</p><!-- ふつうの注 -->"
	if got := string(RedactHTML([]byte(in))); got != in {
		t.Errorf("当たらないコメントを書き換えた: %s", got)
	}
}
