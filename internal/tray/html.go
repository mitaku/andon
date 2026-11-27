package tray

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"andon/internal/redact"
)

// RedactHTML は HTML の文の部分（テキストのノード）と属性の値にだけ伏せる式を当てる（決めたこと 10）。
// <script> と <style> の中身には当てない（CSS の閉じタグごと消える・スクリプトが構文の誤りになる）。
// 当たらなかった所は元のバイトのまま返す（読み直して書き直さない）。
func RedactHTML(body []byte) []byte {
	z := html.NewTokenizer(bytes.NewReader(body))
	var out bytes.Buffer
	raw := "" // 中身を伏せない要素（script・style）の中にいる間の名前
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return out.Bytes()
		}
		chunk := z.Raw()
		switch tt {
		case html.TextToken:
			if raw != "" {
				out.Write(chunk)
				continue
			}
			text := html.UnescapeString(string(chunk))
			if got, n := redact.JSON([]byte(text)); n > 0 {
				out.WriteString(html.EscapeString(string(got)))
				continue
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if tt == html.StartTagToken && (t.DataAtom == atom.Script || t.DataAtom == atom.Style) {
				raw = t.Data
			}
			changed := false
			for i, a := range t.Attr {
				if got, n := redact.JSON([]byte(a.Val)); n > 0 {
					t.Attr[i].Val, changed = string(got), true
				}
			}
			if changed {
				out.WriteString(t.String())
				continue
			}
		case html.EndTagToken:
			if name, _ := z.TagName(); raw != "" && string(name) == raw {
				raw = ""
			}
		}
		out.Write(chunk)
	}
}

// Embedded は Embed の結果。
type Embedded struct {
	Body    []byte
	Left    int      // 埋めずに残した http・https の参照の数（決めたこと 10。報告の面では表示されない）
	Missing []string // 埋めようとして読めなかった手元の参照
}

// Embed は file（HTML）が相対で引いている画像・スタイル・スクリプト・フォント（スタイルの url()）を data: と中身に埋め、
// 1枚で表示できる写しにする（決めたこと 10）。枠の中のリンクが新しいタブで開くよう <head> に <base target="_blank"> を足し、
// # で始まるリンクには target="_self" を足す（ページの中の目次は同じ枠で動く）。
func Embed(file string, body []byte) (*Embedded, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	e := &embedder{dir: filepath.Dir(file)}
	e.walk(doc)
	addBase(doc)
	var out bytes.Buffer
	if err := html.Render(&out, doc); err != nil {
		return nil, err
	}
	if out.Len() > MaxBytes {
		return nil, fmt.Errorf("参照を埋めると大きすぎる（%d バイト、上限 %d）", out.Len(), MaxBytes)
	}
	return &Embedded{Body: out.Bytes(), Left: e.left, Missing: e.missing}, nil
}

type embedder struct {
	dir     string
	left    int
	missing []string
}

func (e *embedder) walk(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling // 埋めると c を置き換える
		e.walk(c)
		c = next
	}
	if n.Type != html.ElementNode {
		return
	}
	switch n.DataAtom {
	case atom.Img, atom.Source, atom.Video, atom.Audio, atom.Input:
		e.dataAttr(n, "src", e.dir)
		if n.DataAtom == atom.Video {
			e.dataAttr(n, "poster", e.dir)
		}
	case atom.Link:
		rel := " " + strings.ToLower(attr(n, "rel")) + " "
		switch {
		case strings.Contains(rel, " stylesheet "):
			e.inlineStylesheet(n)
		case strings.Contains(rel, " icon "):
			e.dataAttr(n, "href", e.dir)
		}
	case atom.Script:
		e.inlineScript(n)
	case atom.Style:
		if c := n.FirstChild; c != nil && c.Type == html.TextNode {
			c.Data = e.css(c.Data, e.dir)
		}
	case atom.A:
		if strings.HasPrefix(attr(n, "href"), "#") {
			setAttr(n, "target", "_self")
		}
	}
	if s, ok := attrOK(n, "style"); ok {
		setAttr(n, "style", e.css(s, e.dir))
	}
}

// local は ref が手元のファイルを指すなら、そのパスを返す。外の参照（http・https・//）は数える。
// data:・blob:・#・mailto: などの scheme を持つ参照は触らない。
func (e *embedder) local(ref, dir string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "#") {
		return "", false
	}
	if strings.HasPrefix(ref, "//") {
		e.left++
		return "", false
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "", false
	}
	if u.Scheme != "" {
		if u.Scheme == "http" || u.Scheme == "https" {
			e.left++
		}
		return "", false
	}
	if u.Path == "" || strings.HasPrefix(u.Path, "/") {
		// サイトの根からの参照は、手元のどこを指すか決められない
		e.missing = append(e.missing, ref)
		return "", false
	}
	return filepath.Join(dir, filepath.FromSlash(u.Path)), true
}

func (e *embedder) read(ref, dir string) (string, []byte, bool) {
	p, ok := e.local(ref, dir)
	if !ok {
		return "", nil, false
	}
	st, err := os.Stat(p)
	if err == nil && !st.Mode().IsRegular() {
		err = fmt.Errorf("ファイルでない")
	}
	var b []byte
	if err == nil {
		b, err = os.ReadFile(p)
	}
	if err != nil {
		e.missing = append(e.missing, ref)
		return "", nil, false
	}
	return p, b, true
}

func (e *embedder) dataAttr(n *html.Node, key, dir string) {
	v, ok := attrOK(n, key)
	if !ok {
		return
	}
	if p, b, ok := e.read(v, dir); ok {
		setAttr(n, key, dataURL(p, b))
	}
}

func (e *embedder) inlineStylesheet(n *html.Node) {
	href, ok := attrOK(n, "href")
	if !ok {
		return
	}
	p, b, ok := e.read(href, e.dir)
	if !ok {
		return
	}
	style := &html.Node{Type: html.ElementNode, Data: "style", DataAtom: atom.Style}
	if m := attr(n, "media"); m != "" {
		style.Attr = []html.Attribute{{Key: "media", Val: m}}
	}
	style.AppendChild(&html.Node{Type: html.TextNode, Data: e.css(string(b), filepath.Dir(p))})
	n.Parent.InsertBefore(style, n)
	n.Parent.RemoveChild(n)
}

func (e *embedder) inlineScript(n *html.Node) {
	src, ok := attrOK(n, "src")
	if !ok {
		return
	}
	_, b, ok := e.read(src, e.dir)
	if !ok {
		return
	}
	delAttr(n, "src")
	for c := n.FirstChild; c != nil; c = n.FirstChild {
		n.RemoveChild(c)
	}
	// </script> が中身に在ると、そこで要素が閉じる
	text := strings.ReplaceAll(string(b), "</script", `<\/script`)
	n.AppendChild(&html.Node{Type: html.TextNode, Data: text})
}

var cssURL = regexp.MustCompile(`url\(\s*(?:"([^"]*)"|'([^']*)'|([^'")\s]+))\s*\)`)

// css は CSS の url() のうち手元のファイルを指すものを data: に埋める。dir は CSS の置き場（相対の参照の起点）。
func (e *embedder) css(s, dir string) string {
	return cssURL.ReplaceAllStringFunc(s, func(m string) string {
		sub := cssURL.FindStringSubmatch(m)
		ref := sub[1] + sub[2] + sub[3]
		p, b, ok := e.read(ref, dir)
		if !ok {
			return m
		}
		return `url("` + dataURL(p, b) + `")`
	})
}

func dataURL(path string, b []byte) string {
	return "data:" + ContentType(path) + ";base64," + base64.StdEncoding.EncodeToString(b)
}

// addBase は <head> に <base target="_blank"> を足す。<base> が在れば target だけを足す。
func addBase(doc *html.Node) {
	head := find(doc, atom.Head)
	if head == nil {
		return
	}
	if b := find(head, atom.Base); b != nil {
		if _, ok := attrOK(b, "target"); !ok {
			setAttr(b, "target", "_blank")
		}
		return
	}
	base := &html.Node{Type: html.ElementNode, Data: "base", DataAtom: atom.Base, Attr: []html.Attribute{{Key: "target", Val: "_blank"}}}
	head.InsertBefore(base, head.FirstChild)
}

func find(n *html.Node, a atom.Atom) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := find(c, a); f != nil {
			return f
		}
	}
	return nil
}

func attrOK(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func attr(n *html.Node, key string) string { v, _ := attrOK(n, key); return v }

func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if a.Namespace == "" && a.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

func delAttr(n *html.Node, key string) {
	out := n.Attr[:0]
	for _, a := range n.Attr {
		if a.Namespace != "" || a.Key != key {
			out = append(out, a)
		}
	}
	n.Attr = out
}

// HTMLTitle は HTML の <title> の文（無ければ空）。
func HTMLTitle(body []byte) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	t := find(doc, atom.Title)
	if t == nil {
		return ""
	}
	var b strings.Builder
	for c := t.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

var mdHeading = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.+?)[ \t#]*$`)

// MarkdownTitle は Markdown の最初の見出し（無ければ空）。コードフェンスの中は見ない。
func MarkdownTitle(body []byte) string {
	fence := ""
	for _, line := range strings.Split(string(body), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			if fence == "" {
				fence = t[:3]
			} else if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if m := mdHeading.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

// ContentType は拡張子から決める形式（決めたこと 22。拡張子から決めるのはセルの側）。知らない拡張子はダウンロードになる形式。
// 表は手で持つ（mime の表はマシンの /etc/mime.types で変わる）。
func ContentType(path string) string {
	if t, ok := types[strings.ToLower(filepath.Ext(path))]; ok {
		return t
	}
	return "application/octet-stream"
}

var types = map[string]string{
	".html": "text/html; charset=utf-8", ".htm": "text/html; charset=utf-8",
	".md": "text/markdown; charset=utf-8", ".markdown": "text/markdown; charset=utf-8",
	".txt": "text/plain; charset=utf-8", ".log": "text/plain; charset=utf-8",
	".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".json": "application/json",
	".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".webp": "image/webp", ".avif": "image/avif", ".pdf": "application/pdf",
	".woff": "font/woff", ".woff2": "font/woff2", ".ttf": "font/ttf", ".otf": "font/otf",
}
