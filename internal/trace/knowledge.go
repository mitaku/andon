package trace

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// 知識の読み（ADR-0061 決めたこと 15・16）: 課題のセッションが読んだセルの wiki のページと、目録を読んだ回数。
// 数えるのは、ファイルを読むツール（Read）の span の file_path と、Bash の span の全文 full_command に、
// wiki の置き場の下のパスが出たものである。ページは wiki の層（entities/・topics/・sources/ の下の .md と overview.md）で、
// 目録（index.md）はページと分けて数える。log.md と元資料（raw/）は数えない。

// WikiDir はセルの wiki の置き場（セルの repo の wiki/。ANDON_WORKSPACE。既定 /workspace。ADR-0016）。
func WikiDir() string {
	ws := os.Getenv("ANDON_WORKSPACE")
	if ws == "" {
		ws = "/workspace"
	}
	return filepath.Join(ws, "wiki")
}

// Knowledge は outcome.json の breakdown.knowledge（schema/outcome.schema.json）。
type Knowledge struct {
	IndexReads int        `json:"index_reads"`
	Pages      []PageRead `json:"pages"`
}

// PageRead は wiki のページ 1 枚の読まれた数（パスは wiki の置き場からの相対パス）。
type PageRead struct {
	Path  string `json:"path"`
	Reads int    `json:"reads"`
}

// wikiPath は wiki の置き場からの相対パスになる文字の並び（.md で終わる。後ろがパスの文字なら別の名前〈a.md.bak・a.mdx〉なので捨てる）。
const wikiPath = `((?:[A-Za-z0-9_.@+-]+/)*[A-Za-z0-9_.@+-]+\.md)([A-Za-z0-9_.@+-]?)`

// relWiki は wiki/ から始まる相対パス（パスの文字でない文字のあと。docs/wiki/ のような別の置き場の wiki/ は数えない）。
var relWiki = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.@+/~-])wiki/` + wikiPath)

// WikiRefs は文 text に出る wiki のページ（相対パスの名前の順。重ならない）と、目録が出たか。
// wiki は置き場の絶対パス。text の中の、置き場の絶対パスの下のパスと、語の切れ目から始まる wiki/ の下のパスを数える。
func WikiRefs(text, wiki string) (pages []string, index bool) {
	seen := map[string]bool{}
	take := func(rel, after string) {
		if after != "" {
			return
		}
		switch k := wikiKind(rel); k {
		case "index":
			index = true
		case "page":
			seen[rel] = true
		}
	}
	if wiki != "" {
		abs := regexp.MustCompile(regexp.QuoteMeta(filepath.Clean(wiki)+"/") + wikiPath)
		for _, m := range abs.FindAllStringSubmatch(text, -1) {
			take(m[1], m[2])
		}
	}
	for _, m := range relWiki.FindAllStringSubmatch(text, -1) {
		take(m[1], m[2])
	}
	for p := range seen {
		pages = append(pages, p)
	}
	sort.Strings(pages)
	return pages, index
}

// wikiKind は wiki の置き場からの相対パスの種類: index（目録）・page（wiki の層のページ）・空（数えない）。
func wikiKind(rel string) string {
	rel = filepath.ToSlash(rel)
	if rel != filepath.ToSlash(filepath.Clean(rel)) || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
		return ""
	}
	switch rel {
	case "index.md":
		return "index"
	case "overview.md":
		return "page"
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || strings.HasPrefix(seg, ".") {
			return "" // 隠しファイルと .. は数えない（schema の path の pattern と同じ線）
		}
	}
	for _, d := range []string{"entities/", "topics/", "sources/"} {
		if strings.HasPrefix(rel, d) && strings.HasSuffix(rel, ".md") {
			return "page"
		}
	}
	return ""
}

// knowledge は span のうち Read と Bash のツールの記録から、知識の読みを数える。1 つのツールの記録の中で同じページが
// 何度出ても 1 回に数える。wiki が空なら、wiki/ から始まる相対パスだけを数える。
func knowledge(spans []*span, wiki string) *Knowledge {
	k := &Knowledge{Pages: []PageRead{}}
	reads := map[string]int{}
	for _, sp := range spans {
		if sp.name != "claude_code.tool" {
			continue
		}
		name, _ := sp.attrs["tool_name"].(string)
		if name == "" {
			name, _ = sp.attrs["gen_ai.tool.name"].(string)
		}
		var pages []string
		var index bool
		switch name {
		case "Read":
			p, _ := sp.attrs["file_path"].(string)
			if p == "" || wiki == "" || !filepath.IsAbs(p) {
				continue
			}
			rel, err := filepath.Rel(filepath.Clean(wiki), filepath.Clean(p))
			if err != nil {
				continue
			}
			switch wikiKind(rel) {
			case "index":
				index = true
			case "page":
				pages = []string{filepath.ToSlash(rel)}
			}
		case "Bash":
			cmd, _ := sp.attrs["full_command"].(string)
			pages, index = WikiRefs(cmd, wiki)
		default:
			continue
		}
		if index {
			k.IndexReads++
		}
		for _, p := range pages {
			reads[p]++
		}
	}
	for p, n := range reads {
		k.Pages = append(k.Pages, PageRead{Path: p, Reads: n})
	}
	sort.Slice(k.Pages, func(i, j int) bool {
		if k.Pages[i].Reads != k.Pages[j].Reads {
			return k.Pages[i].Reads > k.Pages[j].Reads
		}
		return k.Pages[i].Path < k.Pages[j].Path
	})
	return k
}
