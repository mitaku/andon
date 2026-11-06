package dispatcher

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// 離席の判定（docs/dispatcher.md §4）のうち、画面を読む純粋な関数。端末の土台（hc）を打つのは deliver.go。
//
//	Screen       hc attach --observe の ANSI の列から、薄い字も残した画面と薄い字を空白に替えた画面を組み直す
//	InputBox     入力欄（最後の2本の罫線の間）の行を、比べられる形で出す
//	InputState   入力欄が empty・text・draft・unknown のどれか
//	RepliedNone  最後の </news> のあとの答えが <no-reply> だけか

// ruleRe は罫線の行（─ が20字以上）。
var ruleRe = regexp.MustCompile(`^\s*(─){20,}\s*$`)

// screenModes は ?…h/l で許すモード（字に関わらない。47・1047・1049 は画面の切り替え）。
// 端末の土台が待機中の Claude Code の画面を描き直すときに出す列（dispatcher.md §4.3 の 3）。
var screenModes = map[string]bool{
	"1": true, "9": true, "25": true, "47": true, "66": true, "1000": true, "1001": true, "1002": true, "1003": true,
	"1004": true, "1005": true, "1006": true, "1015": true, "1016": true, "1047": true, "1049": true, "2004": true,
	"2026": true, "2031": true, "2048": true,
}

type cell struct {
	ch  rune
	dim bool
}

// Screen は ANSI の列 raw を組み直し、薄い字も残した画面 all と、薄い字（SGR 2）を空白に替えた画面 undim を返す。
// 解釈するのは許可の一覧にある列だけで、一覧に無い列・読み切れない列・dim が分からない間の空白でない字・
// UTF-8 でない・何も描かれないときは ok=false（送らない）。dispatcher.md §4.3。
func Screen(raw []byte) (all, undim string, ok bool) {
	if !utf8.Valid(raw) {
		return "", "", false
	}
	s := string(raw)
	rows := map[int]map[int]cell{}
	row, col := 0, 0
	dim, kept := 0, -1 // dim は 1（薄い）・0（通常）・-1（分からない）。kept は ?1049h の控え（初めは分からない）
	erase := func(r, m int) {
		for c := range rows[r] {
			if m == 2 || (m == 0 && c >= col) || (m == 1 && c <= col) {
				delete(rows[r], c)
			}
		}
	}
	put := func(ch rune) {
		if rows[row] == nil {
			rows[row] = map[int]cell{}
		}
		rows[row][col] = cell{ch, dim == 1}
		col += runeWidth(ch)
	}
	i := 0
	for i < len(s) {
		b := s[i]
		switch {
		case b == 0x1b:
			if i+1 >= len(s) {
				return "", "", false
			}
			switch s[i+1] {
			case '[': // CSI: 引数 0x30-0x3F・中間 0x20-0x2F・終わり 0x40-0x7E
				j := i + 2
				for j < len(s) && s[j] >= 0x30 && s[j] <= 0x3f {
					j++
				}
				p := s[i+2 : j]
				k := j
				for k < len(s) && s[k] >= 0x20 && s[k] <= 0x2f {
					k++
				}
				im := s[j:k]
				if k >= len(s) || s[k] < 0x40 || s[k] > 0x7e {
					return "", "", false
				}
				op := s[k]
				i = k + 1
				if !csi(p, im, op, &row, &col, &dim, &kept, rows, erase) {
					return "", "", false
				}
			case ']': // OSC: BEL か ST で終わる。字に関わらない
				j := i + 2
				for j < len(s) && s[j] != 0x07 && s[j] != 0x1b {
					j++
				}
				switch {
				case j < len(s) && s[j] == 0x07:
					i = j + 1
				case j+1 < len(s) && s[j] == 0x1b && s[j+1] == '\\':
					i = j + 2
				default:
					return "", "", false
				}
			default: // ESC 列: H（タブ位置）と (B（ASCII の字の集合）だけ
				j := i + 1
				for j < len(s) && s[j] >= 0x20 && s[j] <= 0x2f {
					j++
				}
				if j >= len(s) || s[j] < 0x30 || s[j] > 0x7e {
					return "", "", false
				}
				if e := s[i+1 : j+1]; e != "H" && e != "(B" {
					return "", "", false
				}
				i = j + 1
			}
		case b == '\r':
			col = 0
			i++
		case b == '\n':
			row, col = row+1, 0
			i++
		case b < 0x20 || b == 0x7f:
			if b != 0x07 && b != 0x18 { // BEL・CAN だけ（字も位置も変えない）
				return "", "", false
			}
			i++
		default:
			r, n := utf8.DecodeRuneInString(s[i:])
			if dim < 0 && !unicode.IsSpace(r) {
				return "", "", false // dim が分からない間に空白でない字を描いた
			}
			put(r)
			i += n
		}
	}
	if len(rows) == 0 {
		return "", "", false
	}
	maxRow := 0
	for r := range rows {
		maxRow = max(maxRow, r)
	}
	var a, u strings.Builder
	for r := 0; r <= maxRow; r++ {
		cols := make([]int, 0, len(rows[r]))
		for c := range rows[r] {
			cols = append(cols, c)
		}
		sort.Ints(cols)
		var o1, o2 strings.Builder
		c := 0
		for _, k := range cols {
			if k > c {
				gap := strings.Repeat(" ", k-c)
				o1.WriteString(gap)
				o2.WriteString(gap)
			}
			x := rows[r][k]
			w := runeWidth(x.ch)
			o1.WriteRune(x.ch)
			if x.dim {
				o2.WriteString(strings.Repeat(" ", w))
			} else {
				o2.WriteRune(x.ch)
			}
			c = k + w
		}
		a.WriteString(strings.TrimRightFunc(o1.String(), unicode.IsSpace) + "\n")
		u.WriteString(strings.TrimRightFunc(o2.String(), unicode.IsSpace) + "\n")
	}
	return a.String(), u.String(), true
}

// csi は CSI 列を1つ解釈する。許可の一覧に無ければ false。
func csi(p, im string, op byte, row, col, dim, kept *int, rows map[int]map[int]cell, erase func(r, m int)) bool {
	pre := ""
	if p != "" && strings.ContainsRune("<=>?", rune(p[0])) {
		pre, p = p[:1], p[1:]
	}
	for _, c := range p {
		if !(c >= '0' && c <= '9') && c != ';' && c != ':' {
			return false
		}
	}
	if im != "" { // 中間バイトつきは DECSTR（CSI ! p）だけ: SGR と控えを通常に戻す
		if im != "!" || op != 'p' || pre != "" || p != "" {
			return false
		}
		*dim, *kept = 0, 0
		return true
	}
	if pre == "?" {
		if (op != 'h' && op != 'l') || p == "" {
			return false
		}
		alt, m1049 := false, false
		for _, m := range strings.Split(p, ";") {
			if !screenModes[m] {
				return false
			}
			alt = alt || m == "47" || m == "1047" || m == "1049"
			m1049 = m1049 || m == "1049"
		}
		if alt {
			clear(rows)
		}
		if m1049 { // 1049 は h で控え、l で戻す
			if op == 'h' {
				*kept = *dim
			} else {
				*dim = *kept
			}
		}
		return true
	}
	if pre != "" { // キーボードの設定（字に関わらない）
		return (pre == "=" && op == 'u') || (pre == ">" && op == 'm')
	}
	args := strings.FieldsFunc(p, func(r rune) bool { return r == ';' || r == ':' })
	n := 1
	if p != "" && p[0] != ';' && p[0] != ':' && len(args) > 0 {
		n, _ = strconv.Atoi(args[0])
	}
	n = max(n, 1)
	switch op {
	case 'm':
		// ; で区切った1つずつが属性。: の付いた組（38:2::r:g:b・4:3 など）は丸ごと1つで、組の中の数は属性として読まない
		g := strings.Split(p, ";")
		for i := 0; i < len(g); i++ {
			v, _ := strconv.Atoi(strings.SplitN(g[i], ":", 2)[0])
			if v == 38 || v == 48 || v == 58 {
				if !strings.Contains(g[i], ":") {
					if i+1 < len(g) && g[i+1] == "5" {
						i += 2
					} else {
						i += 4
					}
				}
				continue
			}
			switch v {
			case 2:
				*dim = 1
			case 0, 22:
				*dim = 0
			}
		}
	case 'H', 'f':
		*row, *col = n-1, 0
		if parts := strings.Split(p, ";"); len(parts) > 1 && parts[1] != "" {
			c, _ := strconv.Atoi(parts[1])
			*col = max(c, 1) - 1
		}
	case 'G':
		*col = n - 1
	case 'd':
		*row = n - 1
	case 'C':
		*col += n
	case 'D':
		*col = max(*col-n, 0)
	case 'A':
		*row = max(*row-n, 0)
	case 'B':
		*row += n
	case 'E':
		*row, *col = *row+n, 0
	case 'J':
		switch p {
		case "", "0", "1":
			z := 0
			if p == "1" {
				z = 1
			}
			erase(*row, z)
			for r := range rows {
				if (z == 0 && r > *row) || (z == 1 && r < *row) {
					delete(rows, r)
				}
			}
		case "2", "3":
			clear(rows)
		default:
			return false
		}
	case 'K':
		switch p {
		case "", "0":
			erase(*row, 0)
		case "1":
			erase(*row, 1)
		case "2":
			erase(*row, 2)
		default:
			return false
		}
	case 'g': // タブ位置を消す（字に関わらない）
	default:
		return false
	}
	return true
}

// runeWidth は端末での字の幅（東アジアの全角・絵文字は 2）。
func runeWidth(r rune) int {
	switch {
	case r >= 0x1100 && r <= 0x115F, r >= 0x2E80 && r <= 0xA4CF, r >= 0xAC00 && r <= 0xD7A3, r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE30 && r <= 0xFE4F, r >= 0xFF00 && r <= 0xFF60, r >= 0xFFE0 && r <= 0xFFE6, r >= 0x1F300 && r <= 0x1FAFF,
		r >= 0x20000 && r <= 0x3FFFD:
		return 2
	}
	return 1
}

// rules は画面の行と、最後の2本の罫線の行の番号（r2 < r1。無ければ -1）。
func rules(screen string) (lines []string, r2, r1 int) {
	lines = strings.Split(strings.TrimSuffix(screen, "\n"), "\n")
	r2, r1 = -1, -1
	for i, l := range lines {
		if ruleRe.MatchString(l) {
			r2, r1 = r1, i
		}
	}
	return lines, r2, r1
}

func nbsp(s string) string { return strings.ReplaceAll(s, " ", " ") }

// InputBox は入力欄の行（最後の2本の罫線の間）を、比べられる形で出す（NBSP は空白に、行末の空白は除く）。
// 罫線が2本無い・間に行が無ければ ok=false。
func InputBox(screen string) (box []string, ok bool) {
	lines, r2, r1 := rules(screen)
	if r2 < 0 || r1-r2 < 2 {
		return nil, false
	}
	for _, l := range lines[r2+1 : r1] {
		box = append(box, strings.TrimRightFunc(nbsp(l), unicode.IsSpace))
	}
	return box, true
}

// InputState は入力欄の様子（dispatcher.md §4.2）: 2本の罫線の間が ❯ だけの1行なら empty、❯ の後ろに字があれば text、
// 2行以上なら draft、罫線が2本無い・間が ❯ で始まらないなら unknown。
func InputState(screen string) string {
	lines, r2, r1 := rules(screen)
	if r2 < 0 {
		return "unknown"
	}
	switch n := r1 - r2 - 1; {
	case n == 0:
		return "unknown"
	case n > 1:
		return "draft"
	}
	s := strings.TrimSpace(nbsp(lines[r2+1]))
	rest, found := strings.CutPrefix(s, "❯")
	if !found {
		return "unknown"
	}
	if strings.TrimSpace(rest) == "" {
		return "empty"
	}
	return "text"
}

var (
	userLineRe = regexp.MustCompile(`^\s*❯`)
	answerRe   = regexp.MustCompile(`^\s*●\s*`)
	breakRe    = regexp.MustCompile(`^\s*$|^\s*(⎿|✻)`)
)

// RepliedNone は、画面で最後の </news> の行より後に人の行（❯ で始まる行。入力欄を除く）が無く、
// 最後の答え（● で始まる塊）が <no-reply> だけか（dispatcher.md §3.1 の news.replied）。
func RepliedNone(screen string) bool {
	lines, r2, _ := rules(screen)
	end := len(lines) - 1
	if r2 >= 0 {
		end = r2 - 1
	}
	from := -1
	for i := end; i >= 0; i-- {
		if strings.Contains(lines[i], "</news>") {
			from = i
			break
		}
	}
	if from < 0 {
		return false
	}
	ans, in := "", false
	for i := from + 1; i <= end; i++ {
		s := nbsp(lines[i])
		switch {
		case userLineRe.MatchString(s):
			return false
		case answerRe.MatchString(s):
			ans, in = answerRe.ReplaceAllString(s, ""), true
		case breakRe.MatchString(s):
			in = false
		case in:
			ans += " " + strings.TrimLeftFunc(s, unicode.IsSpace)
		}
	}
	return strings.TrimRightFunc(ans, unicode.IsSpace) == "<no-reply>"
}
