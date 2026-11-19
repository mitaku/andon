package ledger

import (
	"fmt"
	"strconv"
	"strings"
)

// 課題の id の読み方（docs/ledger.md の Ids）。台帳の CLI（cmd/kanban）と andon の command が同じものを使う。

// IDDigits は課題の番号の最小の桁数（A-0005）。名前の順が番号の順になる（9999 を超えると番号だけが伸び、名前の順は崩れる）。
const IDDigits = 4

// FormatID は <prefix>-<n>（n は IDDigits 桁に0で埋める）。
func FormatID(prefix string, n int) string { return fmt.Sprintf("%s-%0*d", prefix, IDDigits, n) }

// RoundID は親の課題の k 回目のレビュアーの課題の id（<親>-r<k>。A-0005-r1）。
func RoundID(parent string, k int) string { return fmt.Sprintf("%s-r%d", parent, k) }

// SplitRound はレビュアーの課題の id（<親>-r<回>。A-0005-r1）を親の id と回（1 以上）に分ける。違う形なら ok が false。
func SplitRound(id string) (parent string, round int, ok bool) {
	i := strings.LastIndex(id, "-r")
	if i <= 0 || i+2 == len(id) || strings.Trim(id[i+2:], "0123456789") != "" {
		return "", 0, false
	}
	k, err := strconv.Atoi(id[i+2:])
	if err != nil || k < 1 {
		return "", 0, false
	}
	return id[:i], k, true
}

// CanonID は人が打った id を台帳の形に揃える（A-5・A-05 → A-0005。レビュアーの A-5-r1 → A-0005-r1。回は先頭の0を除く）。
// どちらの形でもなければそのまま。
func CanonID(id string) string {
	if base, k, ok := SplitRound(id); ok {
		return RoundID(canonNumber(base), k)
	}
	return canonNumber(id)
}

// canonNumber は <prefix>-<数字> の CanonID。
func canonNumber(id string) string {
	i := strings.LastIndex(id, "-")
	if i <= 0 || i == len(id)-1 || strings.Trim(id[i+1:], "0123456789") != "" {
		return id
	}
	n, err := strconv.Atoi(id[i+1:])
	if err != nil || n < 1 {
		return id
	}
	return FormatID(id[:i], n)
}

// NumberOf は課題の id の番号（A-0005 は 5。レビュアーの A-0005-r1 も親の 5）。読めなければ 0。
func NumberOf(id string) int {
	if base, _, ok := SplitRound(id); ok {
		id = base
	}
	n, _ := strconv.Atoi(id[strings.LastIndex(id, "-")+1:])
	return n
}

// IDLess は台帳の id の並べ方: 番号の順、レビュアーの課題は親のすぐあとに回の順、残りは名前の順。
func IDLess(a, b string) bool {
	if na, nb := NumberOf(a), NumberOf(b); na != nb {
		return na < nb
	}
	_, ra, _ := SplitRound(a)
	_, rb, _ := SplitRound(b)
	if ra != rb {
		return ra < rb
	}
	return a < b
}
