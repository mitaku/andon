package gate

import (
	"fmt"
	"regexp"
	"strings"
)

// PR のコメントと commit status の説明文は、PR を読む外の人に向けた文（ADR-0026 決めたこと 9）。
// ゲートの記録（gate/<head>.json の reasons・tier_reasons）は台帳の読み手の文で、変えない。
// ここでは記録から外向けの文を作るだけで、記録の文を外へそのまま写さない。
// 頭の「andon gate: 」は動かさない（dispatcher がこの頭でゲートのコメントを見分け、人の手直しから除く）。

// CommentPrefix はゲートが PR に書くコメントと commit status の説明文の頭（dispatcher の gateCommentPrefix と同じ）。
const CommentPrefix = "andon gate: "

// headline は判定ごとの見出し（外の人が読む文）。
func headline(decision string) string {
	switch decision {
	case "approve":
		return "確かめが通った変更です"
	case "fix":
		return "作った側が直してから出し直す変更です"
	case "needs_human":
		return "人の判断が要る変更です"
	}
	return "判定できなかった変更です"
}

// tierText は変更の区分（記録の tier）を外の人が読む文にする。
func tierText(tier string) string {
	switch tier {
	case "A":
		return "自動の確かめだけで足りる変更"
	case "B":
		return "レビューが要る変更"
	case "C":
		return "人が見る変更"
	}
	return "区分の分からない変更"
}

// StatusDescription は commit status（andon/gate）の説明文。
func StatusDescription(rec *Record) string {
	if rec.Decision == "approve" {
		return CommentPrefix + headline(rec.Decision)
	}
	return CommentPrefix + headline(rec.Decision) + "（" + tierText(rec.Tier) + "）"
}

// CommentBody は PR に書くコメントの本文。
func CommentBody(rec *Record) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s**%s**（commit %s）\n\n", CommentPrefix, headline(rec.Decision), rec.Head)
	for _, r := range rec.Reasons {
		fmt.Fprintf(&b, "- 理由: %s\n", outsideReason(r))
	}
	tr := make([]string, len(rec.TierReasons))
	for i, r := range rec.TierReasons {
		tr[i] = outsideTierReason(r)
	}
	if len(tr) > 0 {
		fmt.Fprintf(&b, "- 変更の区分: %s（%s）。%d 行\n", tierText(rec.Tier), strings.Join(tr, "、"), rec.ChangedLines)
	} else {
		fmt.Fprintf(&b, "- 変更の区分: %s。%d 行\n", tierText(rec.Tier), rec.ChangedLines)
	}
	for _, c := range rec.Checks {
		fmt.Fprintf(&b, "- 検査 `%s`: %s\n", c.Cmd, map[bool]string{true: "通った", false: "落ちた"}[c.OK])
	}
	fmt.Fprintf(&b, "- CI: %s（%s 時点）\n", checksText(rec.GitHubChecks), rec.At)
	fmt.Fprintf(&b, "- レビュー: %s\n", reviewText(rec))
	if rec.Delegation.OK {
		b.WriteString("- 自動でマージしてよい範囲: 中\n")
	} else {
		b.WriteString("- 自動でマージしてよい範囲: 外（人がマージする）\n")
	}
	return b.String()
}

func checksText(g GitHubChecks) string {
	names := ""
	if len(g.Names) > 0 {
		names = "。" + strings.Join(g.Names, ", ")
	}
	switch g.State {
	case "success":
		return "通った" + names
	case "failure":
		return "落ちた" + names
	case "pending":
		return "実行中" + names
	case "none":
		return "無い"
	}
	return "結果を判定できないものがある" + names
}

func reviewText(rec *Record) string {
	r := rec.Review
	switch r.State {
	case "off":
		return "要らない"
	case "missing":
		return "まだ無い"
	case "stale":
		return "最新の commit を見ていない"
	}
	return fmt.Sprintf("済み（直す重さの指摘が残り %d 件・見送り %d 件）", r.MustFixOpen, r.Waived)
}

type rewrite struct {
	re *regexp.Regexp
	to func(m []string) string
}

func fixed(s string) func([]string) string { return func([]string) string { return s } }

// reasonRewrites は記録の reasons（Decide と run が書く文）を外向けの文に直す表。
// 記録の文を変えたら、ここと TestCommentReasons の両方を直す。
var reasonRewrites = []rewrite{
	{regexp.MustCompile(`^台帳の課題が review でない（.*）$`), fixed("作業がまだ確認待ちになっていない")},
	{regexp.MustCompile(`^PR の branch（(.*)）が issue\.yaml の branch と違う$`), func(m []string) string {
		return "PR の branch（" + m[1] + "）が、作業を始めたときの branch と違う"
	}},
	{regexp.MustCompile(`^report\.json の commit が PR の head と違う`), fixed("作った側の報告が PR の最新の commit を指していない")},
	{regexp.MustCompile(`^PR が open でない・draft$`), fixed("PR が開いていない、または draft")},
	{regexp.MustCompile(`^files の件数が changedFiles と合わない`), fixed("変更されたファイルの一覧を読み切れなかった")},
	{regexp.MustCompile(`^conflict$`), fixed("既定ブランチと衝突している")},
	{regexp.MustCompile(`^fork の PR$`), fixed("fork からの PR")},
	{regexp.MustCompile(`^base が既定ブランチでない$`), fixed("PR の向き先が既定ブランチでない")},
	{regexp.MustCompile(`^作者がセルの gh アカウントでない$`), fixed("PR の作者が、ゲートを回しているアカウントでない")},
	{regexp.MustCompile(`^issue\.yaml の review・must_fix`), fixed("作業の設定（レビューの要否・直す指摘の重さ）を確かめられない")},
	{regexp.MustCompile(`^条件表の repos にこの repo が無い`), fixed("この repo の確かめ方が設定に無い（検査を回せない）")},
	{regexp.MustCompile(`^delegation: merge=.* < tier .*$`), fixed("自動でマージしてよい範囲を超える変更")},
	{regexp.MustCompile(`^check: (.*)$`), func(m []string) string { return "検査が落ちた: `" + m[1] + "`" }},
	{regexp.MustCompile(`^GitHub の checks が落ちた: (.*)$`), func(m []string) string { return "CI が落ちた: " + m[1] }},
	{regexp.MustCompile(`^GitHub の checks に通すか決められない状態がある: (.*)$`), func(m []string) string {
		return "CI に結果を判定できないものがある: " + m[1]
	}},
	{regexp.MustCompile(`^review missing: `), fixed("レビューがまだ無い")},
	{regexp.MustCompile(`^review stale: `), fixed("レビューが最新の commit を見ていない")},
	{regexp.MustCompile(`^Must fix 以上が ([0-9]+) 件、同じ head で2度目`), func(m []string) string {
		return "直す重さの指摘が " + m[1] + " 件残っていて、作った側は直さないと判断した"
	}},
	{regexp.MustCompile(`^Must fix 以上が ([0-9]+) 件$`), func(m []string) string { return "直す重さの指摘が " + m[1] + " 件残っている" }},
	{regexp.MustCompile(`^Must fix 以上を ([0-9]+) 件見送った`), func(m []string) string {
		return "直す重さの指摘を " + m[1] + " 件見送った（レビューの判断を覆した）"
	}},
	{regexp.MustCompile(`^受け入れ条件の fail が ([0-9]+) 件$`), func(m []string) string { return "受け入れ条件のうち " + m[1] + " 件が通っていない" }},
	{regexp.MustCompile(`^screen_paths: (.*) に当たるが画面の証拠（添付）が無い$`), func(m []string) string {
		return "画面に触れる変更（" + m[1] + "）なのに、画面の写しの添付が無い"
	}},
	{regexp.MustCompile(`^GitHub の checks がまだ走っている: (.*)$`), func(m []string) string { return "CI がまだ走っている: " + m[1] }},
	{regexp.MustCompile(`^GitHub の checks が1つも無いまま ([0-9]+) 分過ぎた`), func(m []string) string {
		return "CI が 1 つも無いまま " + m[1] + " 分過ぎた"
	}},
	{regexp.MustCompile(`^GitHub の checks がまだ1つも無い$`), fixed("CI がまだ 1 つも無い")},
}

// tierReasonRewrites は記録の tier_reasons（Classify と FromReport が書く文）を外向けの文に直す表。
var tierReasonRewrites = []rewrite{
	{regexp.MustCompile(`^path (.*)（tier_overrides）$`), func(m []string) string { return "この repo で個別に決めたファイル " + m[1] }},
	{regexp.MustCompile(`^path (.*)$`), func(m []string) string { return "人が見ると決めたファイル " + m[1] }},
	{regexp.MustCompile(`^実行面 (.*)$`), func(m []string) string { return "動きに関わるファイル " + m[1] }},
	{regexp.MustCompile(`^段 A 許可リスト外 (.*)$`), func(m []string) string { return "自動の確かめだけで足りるファイルの外 " + m[1] }},
	{regexp.MustCompile(`^追加行 (.*)$`), func(m []string) string { return "人が見ると決めた形の行を足した（`" + m[1] + "`）" }},
	{regexp.MustCompile(`^([0-9]+) 行（size_(?:human|review)_lines ([0-9]+) 超）$`), func(m []string) string {
		return m[1] + " 行（" + m[2] + " 行を超える）"
	}},
	{regexp.MustCompile(`^review: on$`), fixed("レビューを入れると決めた作業")},
	{regexp.MustCompile(`^受け入れ条件の manual が ([0-9]+) 件`), func(m []string) string { return "人が確かめる受け入れ条件が " + m[1] + " 件" }},
	{regexp.MustCompile(`^` + regexp.QuoteMeta(OneWayReason) + `$`), fixed("戻せない変更だと作った側が申告した")},
}

// otherReason は表に無い記録の文の代わり（記録の文は内輪の語を含みうるので、外へそのまま出さない）。
const otherReason = "そのほかの理由"

func outsideReason(r string) string     { return apply(reasonRewrites, r) }
func outsideTierReason(r string) string { return apply(tierReasonRewrites, r) }

func apply(table []rewrite, r string) string {
	for _, w := range table {
		if m := w.re.FindStringSubmatch(r); m != nil {
			return w.to(m)
		}
	}
	return otherReason
}
