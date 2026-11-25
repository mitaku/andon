package measure

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"andon/internal/trace"
)

// 知識の節（ADR-0061 決めたこと 15・17。docs/contract.md §13）。期間に片付いた担当の課題（issues[] と同じ顔ぶれ）について、
// 依頼書に wiki のページのパスが出る課題の数（brief.md）、wiki のページを 1 枚以上読んだ課題の数とページごとの読まれた数
// （outcome.json の breakdown.knowledge）、知識の欠けの申告の数（report.json の gaps[]）を出す。
// 読むための数で、gate の判定・委譲・wiki のページ・skill の文を、この数から変えない（決めたこと 19）。

// KnowledgeSection は知識の節。
type KnowledgeSection struct {
	Issues  int             `json:"issues"`  // 期間に片付いた担当の課題
	Briefed int             `json:"briefed"` // そのうち依頼書に wiki のページのパスが出る課題
	Read    KnowledgeRead   `json:"read"`
	Pages   []KnowledgePage `json:"pages"`  // 読まれた数の多いページの上から 10 個
	Unread  []string        `json:"unread"` // 期間に 1 回も読まれなかった Topics と Entities のページ（数えられないときは null）
	Gaps    KnowledgeGaps   `json:"gaps"`
}

// KnowledgeRead は読んだ課題の数。読みの記録の無い課題（内訳が無いか null・内訳に knowledge が無い）は、読まなかった課題と分ける。
type KnowledgeRead struct {
	Issues    int `json:"issues"`     // wiki のページを 1 枚以上読んだ課題（目録だけの課題は入れない）
	IndexOnly int `json:"index_only"` // 目録だけを読んだ課題
	NoRecord  int `json:"no_record"`  // 読みの記録の無い課題
}

// KnowledgePage はページ 1 枚の読まれた数（パスは wiki の置き場からの相対パス）。
type KnowledgePage struct {
	Path   string `json:"path"`
	Reads  int    `json:"reads"`  // 読んだ回数の合計
	Issues int    `json:"issues"` // 読んだ課題の数
}

// KnowledgeGaps は申告の数（種類ごと・先ごと）と、同じ先への申告が 2 件以上ある繰り返し。
type KnowledgeGaps struct {
	Count    int        `json:"count"`
	Issues   int        `json:"issues"` // 申告を 1 件以上書いた課題
	ByKind   GapKinds   `json:"by_kind"`
	Wheres   []GapWhere `json:"wheres"`   // 先ごと（数の多い順、同じなら先の名前の順）
	Repeated []GapWhere `json:"repeated"` // そのうち 2 件以上の先（先が言えない unknown は入れない）
}

// GapKinds は申告の種類ごとの数。
type GapKinds struct {
	Missing    int `json:"missing"`
	Wrong      int `json:"wrong"`
	HardToFind int `json:"hard_to_find"`
}

// GapWhere は 1 つの先への申告の数と、書いた課題（id の順。重ならない）。
type GapWhere struct {
	Where  string   `json:"where"`
	Count  int      `json:"count"`
	Issues []string `json:"issues"`
}

// knowledge は知識の節を組む。workers は期間に片付いた担当の課題の id（issues[] の順）、bs は片付いた課題の内訳。
func (r *reader) knowledge(o Options, workers []string, byID map[string]*issue, bs map[string]*trace.Breakdown) KnowledgeSection {
	s := KnowledgeSection{Pages: []KnowledgePage{}, Gaps: KnowledgeGaps{Wheres: []GapWhere{}, Repeated: []GapWhere{}}}
	pages := map[string]*pageAcc{}
	wheres := map[string]*GapWhere{}
	records := 0
	for _, id := range workers {
		s.Issues++
		dir := filepath.Join(r.dir, "issues", id)
		if b, ok := r.read(filepath.Join(dir, "brief.md")); ok {
			if ps, _ := trace.WikiRefs(string(b), o.Wiki); len(ps) > 0 {
				s.Briefed++
			}
		}
		switch b := bs[id]; {
		case b == nil || b.Knowledge == nil:
			s.Read.NoRecord++
		default:
			records++
			read := false
			for _, p := range b.Knowledge.Pages {
				if p.Reads <= 0 {
					continue
				}
				read = true
				a := pages[p.Path]
				if a == nil {
					a = &pageAcc{}
					pages[p.Path] = a
				}
				a.reads += p.Reads
				a.issues++
			}
			switch {
			case read:
				s.Read.Issues++
			case b.Knowledge.IndexReads > 0:
				s.Read.IndexOnly++
			}
		}
		gs := byID[id].gaps
		if len(gs) > 0 {
			s.Gaps.Issues++
		}
		for _, g := range gs {
			s.Gaps.Count++
			switch g.Kind {
			case "missing":
				s.Gaps.ByKind.Missing++
			case "wrong":
				s.Gaps.ByKind.Wrong++
			case "hard_to_find":
				s.Gaps.ByKind.HardToFind++
			}
			w := gapWhere(g.Where)
			x := wheres[w]
			if x == nil {
				x = &GapWhere{Where: w, Issues: []string{}}
				wheres[w] = x
			}
			x.Count++
			if n := len(x.Issues); n == 0 || x.Issues[n-1] != id {
				x.Issues = append(x.Issues, id)
			}
		}
	}
	for p, a := range pages {
		s.Pages = append(s.Pages, KnowledgePage{Path: p, Reads: a.reads, Issues: a.issues})
	}
	sort.Slice(s.Pages, func(i, j int) bool {
		if s.Pages[i].Reads != s.Pages[j].Reads {
			return s.Pages[i].Reads > s.Pages[j].Reads
		}
		return s.Pages[i].Path < s.Pages[j].Path
	})
	for _, x := range wheres {
		s.Gaps.Wheres = append(s.Gaps.Wheres, *x)
	}
	sort.Slice(s.Gaps.Wheres, func(i, j int) bool {
		if s.Gaps.Wheres[i].Count != s.Gaps.Wheres[j].Count {
			return s.Gaps.Wheres[i].Count > s.Gaps.Wheres[j].Count
		}
		return s.Gaps.Wheres[i].Where < s.Gaps.Wheres[j].Where
	})
	for _, x := range s.Gaps.Wheres {
		if x.Count >= 2 && x.Where != "unknown" {
			s.Gaps.Repeated = append(s.Gaps.Repeated, x)
		}
	}
	// 読まれなかったページは、読みの記録が 1 件も無い期間には出さない（全部が読まれなかったように見えるため。決めたこと 15 の「記録なし」）
	if records > 0 {
		s.Unread = r.unreadPages(o.Wiki, pages)
	}
	if len(s.Pages) > 10 {
		s.Pages = s.Pages[:10]
	}
	return s
}

// gapWhere は申告の先を比べる形にする（前後の空白と先頭の ./ を除き、パスを整える。トリアージと同じ）。空と unknown は unknown。
func gapWhere(w string) string {
	w = strings.TrimSpace(w)
	if w == "" || w == "unknown" {
		return "unknown"
	}
	return path.Clean(strings.TrimPrefix(w, "./"))
}

type gap struct {
	Kind  string `json:"kind"`
	Where string `json:"where"`
}

// gaps は report.json（p の中身 b）の gaps[]。gaps が無ければ空。種類が 3 つのどれでもない申告は飛ばした記録に出して数えない。
// report() が読んだ中身を受ける（読めない report.json を飛ばした記録に 2 回出さない）。
func (r *reader) gaps(p string, b []byte) []gap {
	var v struct {
		Gaps []json.RawMessage `json:"gaps"`
	}
	if json.Unmarshal(b, &v) != nil {
		return nil
	}
	var out []gap
	for i, raw := range v.Gaps {
		var g gap
		if json.Unmarshal(raw, &g) != nil || (g.Kind != "missing" && g.Kind != "wrong" && g.Kind != "hard_to_find") {
			r.skip(r.rel(p), 0, "gaps[%d] の kind が missing・wrong・hard_to_find のどれでもない。この申告は数えていない", i)
			continue
		}
		out = append(out, g)
	}
	return out
}

// unreadPages は wiki の置き場の topics/ と entities/ の下の .md のうち、pages に無いもの（相対パスの名前の順）。
// 置き場が指定されていないか、置き場が無ければ nil（数えられない）。topics/ と entities/ が無ければ空。
func (r *reader) unreadPages(wiki string, read map[string]*pageAcc) []string {
	if wiki == "" {
		return nil
	}
	if _, ok := r.stat(wiki); !ok {
		return nil
	}
	out := []string{}
	for _, layer := range []string{"entities", "topics"} {
		root := filepath.Join(wiki, layer)
		if _, ok := r.stat(root); !ok {
			continue
		}
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				r.skip("wiki/"+layer, 0, "読めない（%s）。読まれなかったページの一覧に入っていないページがある", pathErr(err))
				return fs.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}
			rel, err := filepath.Rel(wiki, p)
			if err != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if read[rel] == nil {
				out = append(out, rel)
			}
			return nil
		})
	}
	sort.Strings(out)
	return out
}

// stat は置き場が在るディレクトリか。無ければ黙って false、ほかの理由で見られなければ飛ばした記録に出して false。
func (r *reader) stat(dir string) (fs.FileInfo, bool) {
	fi, err := os.Stat(dir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			r.skip("wiki", 0, "置き場を見られない（%s）", pathErr(err))
		}
		return nil, false
	}
	return fi, fi.IsDir()
}

// knowledgeTable は知識の節。
func knowledgeTable(w io.Writer, s KnowledgeSection) {
	fmt.Fprintln(w, "\n知識（期間に片付いた担当の課題。依頼書は brief.md、読みは outcome.json の breakdown.knowledge、申告は report.json の gaps）")
	fmt.Fprintf(w, "  依頼書に wiki のページを載せた課題 %d件 / %d件\n", s.Briefed, s.Issues)
	fmt.Fprintf(w, "  wiki のページを読んだ課題      %d件・目録だけ %d件・読みの記録なし %d件\n", s.Read.Issues, s.Read.IndexOnly, s.Read.NoRecord)
	fmt.Fprintln(w, "  読まれた数の多いページ（上から 10 個）")
	if len(s.Pages) == 0 {
		fmt.Fprintf(w, "    %s\n", none)
	}
	for _, p := range s.Pages {
		fmt.Fprintf(w, "    %-36s %d回・%d件\n", p.Path, p.Reads, p.Issues)
	}
	if s.Unread == nil {
		fmt.Fprintf(w, "  1 回も読まれなかった Topics と Entities のページ  %s\n", none)
	} else {
		fmt.Fprintf(w, "  1 回も読まれなかった Topics と Entities のページ  %d枚\n", len(s.Unread))
		for _, p := range s.Unread {
			fmt.Fprintf(w, "    %s\n", p)
		}
	}
	g := s.Gaps
	fmt.Fprintf(w, "  知識の欠けの申告               %d件（%d件の課題。無かった %d・誤っていた %d・辿り着くのが遅れた %d）\n",
		g.Count, g.Issues, g.ByKind.Missing, g.ByKind.Wrong, g.ByKind.HardToFind)
	for _, x := range g.Wheres {
		mark := ""
		if x.Count >= 2 && x.Where != "unknown" {
			mark = "（繰り返し）"
		}
		fmt.Fprintf(w, "    %s %d件%s %s\n", x.Where, x.Count, mark, strings.Join(x.Issues, "・"))
	}
}

// pathErr は誤りの理由だけ（飛ばした記録の理由に絶対パスを書かない）。
func pathErr(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return "読み取りの誤り"
}

// pageAcc はページごとの読んだ回数と課題の数。
type pageAcc struct{ reads, issues int }
