package digest

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"andon/internal/contract"
	"andon/internal/ledger"
	"andon/internal/redact"
)

// Text は担当が書く JSON（schema/digest.schema.json。docs/contract.md §27）。build が照らして digest.json に書く。
type Text struct {
	Headline []string      `json:"headline"`
	Themes   []Theme       `json:"themes"`
	Concerns []TextConcern `json:"concerns"`
}

// Theme はテーマ1つ。
type Theme struct {
	Title  string   `json:"title"`
	Body   []string `json:"body"`
	Issues []string `json:"issues"`
	Figure *Figure  `json:"figure,omitempty"`
}

// Figure は担当が中身を書く図（決めたこと 9。before_after か steps）。
type Figure struct {
	Type   string   `json:"type"`
	Before []string `json:"before,omitempty"`
	After  []string `json:"after,omitempty"`
	Steps  []string `json:"steps,omitempty"`
	Focus  int      `json:"focus,omitempty"`
}

// TextConcern は担当が根拠つきで添える気になること。
type TextConcern struct {
	Text   string   `json:"text"`
	Issues []string `json:"issues"`
}

// 長さの決まり（決めたこと 3 の細部・決めたこと 9）。schema が下限を、ここが上限を見る（検査器の lint は maxItems を読まない）。
const (
	headlineMin  = 2
	headlineMax  = 5
	themesMax    = 8
	themeBodyMax = 3
	concernsMax  = 3
	beforeMax    = 3
	stepsMax     = 6
	htmlMax      = 1 << 20 // 1 MiB（決めたこと 7）
	messageMax   = 3000    // 短い文の上限（字。決めたこと 13）
	messageTop   = 5       // 短い文に並べる待っているもの
)

//go:embed digest.html.tmpl
var pageTmpl string

var page = template.Must(template.New("digest").Funcs(template.FuncMap{"inc": func(i int) int { return i + 1 }}).Parse(pageTmpl))

// Build は課題 id の入力と担当の JSON b から digest.json・digest.html・message.txt を書き、書いたパスを返す。
// 照らして通らなければ理由を1行ずつ返し、何も書かない。
func Build(env Env, id string, b []byte) ([]string, []string) {
	in, err := readInput(env.Root, id)
	if err != nil {
		return nil, []string{err.Error()}
	}
	if err := contract.SchemaDigest.Check(b); err != nil {
		return nil, []string{"担当の JSON が schema/digest.schema.json に合わない: " + err.Error()}
	}
	var text Text
	if err := contract.Decode(b, &text); err != nil {
		return nil, []string{"担当の JSON が読めない: " + err.Error()}
	}
	loc := location(env.Root)
	html, errs := compose(in, &text, loc)
	if len(errs) > 0 {
		return nil, errs
	}
	msg := Message(&text, in, loc, filesURL(env, in))
	dj, err := contract.Encode(contract.SchemaDigest, text)
	if err != nil {
		return nil, []string{"digest.json が schema/digest.schema.json に合わない（書かない）: " + err.Error()}
	}
	d := dir(env.Root, id)
	files := []struct {
		name string
		b    []byte
	}{{digestFile, dj}, {htmlFile, html}, {messageFile, []byte(msg)}}
	// 3つとも一時ファイルに書いてから置き換える（途中で落ちても、前の組の一部だけが新しくなる所を小さくする）
	var paths []string
	for _, f := range files {
		p := filepath.Join(d, f.name)
		if err := os.WriteFile(p+".tmp", f.b, 0o644); err != nil {
			return nil, []string{err.Error()}
		}
		paths = append(paths, p)
	}
	for _, p := range paths {
		if err := os.Rename(p+".tmp", p); err != nil {
			return nil, []string{err.Error()}
		}
	}
	return paths, nil
}

// compose は担当の JSON を入力に照らし、秘密の形を伏せてから HTML に組み立てる（in と text は伏せた形に変わる）。
// build と send の両方が通る。send は組み立て直したものと同じ HTML しか送らない（決めたこと 12 の守りの限り）。
func compose(in *Input, text *Text, loc *Loc) ([]byte, []string) {
	if errs := Check(text, in); len(errs) > 0 {
		return nil, errs
	}
	// 組み立てる前に、入力と担当の JSON の文字列から秘密の形を伏せる（決めたこと 17）
	if err := redactInto(in); err != nil {
		return nil, []string{err.Error()}
	}
	if err := redactInto(text); err != nil {
		return nil, []string{err.Error()}
	}
	html, err := Render(text, in, loc)
	if err != nil {
		return nil, []string{err.Error()}
	}
	if probs := CheckHTML(html); len(probs) > 0 {
		return nil, probs
	}
	return html, nil
}

func redactInto[T any](v *T) error {
	b, _ := redact.JSON(contract.Marshal(v))
	var out T
	if err := contract.Decode(b, &out); err != nil {
		return fmt.Errorf("秘密の形を伏せたあとが読めない: %v", err)
	}
	*v = out
	return nil
}

// Check は担当の JSON を入力と長さの決まりに照らし、通らない理由を1行ずつ返す（決めたこと 3 の細部）。
func Check(t *Text, in *Input) []string {
	var errs []string
	bad := func(format string, a ...any) { errs = append(errs, fmt.Sprintf(format, a...)) }
	// テーマは期間の課題（issues）だけ、気になることはそれに加えて、command が見つけた気になることの課題と待っている課題を指せる
	known, watched := map[string]bool{}, map[string]bool{}
	for _, x := range in.Issues {
		known[x.ID], watched[x.ID] = true, true
	}
	for _, c := range in.Concerns {
		if c.Issue != nil {
			watched[*c.Issue] = true
		}
	}
	for _, it := range in.Waiting.Items {
		watched[it.ID] = true
	}
	refs := func(where string, ids []string) {
		for _, id := range ids {
			if !known[id] {
				bad("%s が入力に無い課題 %s を指す（input.json の issues にある課題だけを指す）", where, id)
			}
		}
	}
	concernRefs := func(where string, ids []string) {
		for _, id := range ids {
			if !watched[id] {
				bad("%s が入力に無い課題 %s を指す（input.json の issues・concerns・waiting にある課題だけを指す）", where, id)
			}
		}
	}
	if n := len(t.Headline); n < headlineMin || n > headlineMax {
		bad("要旨が %d 文（%d〜%d 文にする）", n, headlineMin, headlineMax)
	}
	if n := len(t.Themes); n > themesMax {
		bad("テーマが %d（%d まで）", n, themesMax)
	}
	for i, th := range t.Themes {
		where := fmt.Sprintf("テーマ %d（%s）", i+1, th.Title)
		if n := len(th.Body); n > themeBodyMax {
			bad("%s の文が %d 文（%d 文まで）", where, n, themeBodyMax)
		}
		refs(where, th.Issues)
		if f := th.Figure; f != nil {
			switch f.Type {
			case "before_after":
				if len(f.Before) > beforeMax || len(f.After) > beforeMax {
					bad("%s の図 before_after の行が多い（前も後も %d 行まで）", where, beforeMax)
				}
			case "steps":
				if len(f.Steps) > stepsMax {
					bad("%s の図 steps が %d 段（%d 段まで）", where, len(f.Steps), stepsMax)
				}
				if f.Focus > len(f.Steps) {
					bad("%s の図 steps の focus %d が段の数 %d を超える", where, f.Focus, len(f.Steps))
				}
			default:
				bad("%s の図の型 %q を知らない（before_after か steps）", where, f.Type)
			}
		}
	}
	if n := len(t.Concerns); n > concernsMax {
		bad("気になることが %d（%d まで）", n, concernsMax)
	}
	for i, c := range t.Concerns {
		concernRefs(fmt.Sprintf("気になること %d", i+1), c.Issues)
	}
	return errs
}

var (
	tagRe = regexp.MustCompile(`(?is)<([a-z][a-z0-9:-]*)\b([^>]*)>`)
	refRe = regexp.MustCompile(`(?is)\b(?:xlink:)?(?:src|href)\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	urlRe = regexp.MustCompile(`(?i)url\s*\(`)
	// url( を見るのはスタイルの中（<style> の中身と style 属性）だけ。本文の「curl (」には当てない
	styleRe = regexp.MustCompile(`(?is)<style\b[^>]*>(.*?)</style>|\bstyle\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	svgRe   = regexp.MustCompile(`(?i)<svg\b`)
	scrRe   = regexp.MustCompile(`(?i)<script\b`)
	linkRe  = regexp.MustCompile(`(?i)<link\b`)
)

// CheckHTML は組み立てた HTML が依存の無い単一のファイルかを確かめ、問題を1行ずつ返す（決めたこと 7 の細部）。
// SVG の図が1つ以上あること、<script>・<link> が無いこと、<a> のほかに外を指す src・href が無いこと、スタイルに url( が無いこと、
// html/template が値を捨てた印 ZgotmplZ が無いこと、1 MiB より小さいこと。
func CheckHTML(b []byte) []string {
	var probs []string
	if !svgRe.Match(b) {
		probs = append(probs, "HTML に SVG の図が1つも無い")
	}
	if scrRe.Match(b) {
		probs = append(probs, "HTML に <script がある")
	}
	if linkRe.Match(b) {
		probs = append(probs, "HTML に <link がある")
	}
	for _, m := range tagRe.FindAllSubmatch(b, -1) {
		if strings.EqualFold(string(m[1]), "a") {
			continue
		}
		for _, r := range refRe.FindAllSubmatch(m[2], -1) {
			v := strings.Trim(string(r[1]), `"'`)
			if !strings.HasPrefix(v, "#") {
				probs = append(probs, fmt.Sprintf("HTML の <%s> が外を指す: %s", m[1], v))
			}
		}
	}
	for _, m := range styleRe.FindAllSubmatch(b, -1) {
		if urlRe.Match(m[1]) || urlRe.Match(m[2]) {
			probs = append(probs, "HTML のスタイルに url( がある")
			break
		}
	}
	if bytes.Contains(b, []byte("ZgotmplZ")) {
		probs = append(probs, "HTML に ZgotmplZ がある（html/template が値を捨てた）")
	}
	if len(b) >= htmlMax {
		probs = append(probs, fmt.Sprintf("HTML が %d バイト（%d バイトより小さくする）", len(b), htmlMax))
	}
	return probs
}

// Message は Slack などに添える短い文（決めたこと 13 の細部）。装飾の記法を使わず、messageMax 字まで。
// 超えたら待っているものの行を下から削り、削った数を出す。行が無くても超えるなら前の段を切り、数と URL の行は残す。
// タイトルや文の < > は全角にする（Slack が解釈すると <!channel> で全員を呼び、<URL|文> でリンクに化ける）。
func Message(t *Text, in *Input, loc *Loc, url string) string {
	head := []string{
		fmt.Sprintf("%s（%s）", in.Title, in.Project),
		fmt.Sprintf("期間: %s 〜 %s（%s）", loc.full(in.Since), loc.full(in.Until), loc.name),
	}
	head = append(head, t.Headline...)
	counts := map[string]int{}
	for _, it := range in.Waiting.Items {
		counts[it.Kind]++
	}
	count := fmt.Sprintf("オーナーを待っているもの: %d 件（マージの依頼 %d・判断 %d・手作業 %d）",
		len(in.Waiting.Items), counts["merge"], counts["decision"], counts["task"])
	var rows []string
	for i, it := range in.Waiting.Items {
		if i == messageTop {
			break
		}
		line := fmt.Sprintf("- %s %s %s", kindName[it.Kind], it.ID, it.Title)
		if it.PR != "" {
			line += " " + it.PR
		}
		rows = append(rows, line)
	}
	merged, auto := 0, 0
	for _, x := range in.Issues {
		if x.MergedBy != nil {
			merged++
			if *x.MergedBy == "dispatcher" {
				auto++
			}
		}
	}
	tail := []string{fmt.Sprintf("マージした PR: %d 本（自動 %d 本）", merged, auto), "全文: " + url}
	for {
		keep := append([]string{count}, rows...)
		if omitted := len(in.Waiting.Items) - len(rows); omitted > 0 {
			keep = append(keep, fmt.Sprintf("（ほか %d 件は全文に載せた）", omitted))
		}
		shown := plainText.Replace(strings.Join(head, "\n") + "\n")
		end := plainText.Replace(strings.Join(keep, "\n")+"\n") + strings.Join(tail, "\n") + "\n"
		room := messageMax - utf8.RuneCountInString(end)
		if utf8.RuneCountInString(shown) <= room {
			return shown + end
		}
		if len(rows) == 0 {
			return truncateRunes(shown, room) + end
		}
		rows = rows[:len(rows)-1]
	}
}

var plainText = strings.NewReplacer("<", "＜", ">", "＞")

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "\n"
}

var kindName = map[string]string{"merge": "マージの依頼", "decision": "判断", "task": "手作業"}

// location は kanban の timezone（無ければ UTC）。
func location(root string) *Loc {
	return newLoc(ledger.At(root).Timezone())
}
