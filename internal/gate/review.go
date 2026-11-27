package gate

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/ledger"
)

// ReviewRequired は独立レビューが要るか。gate の L3 と、レビューを受ける側（andon issue review-request・ready・
// dispatcher のレビュアーの起動）は、この1つの関数で決める（条件を食い違わせない）。
// 差分がレベル B・C なら、課題の review が off でも要る。review は issue.yaml の予約キー（Chief of Staff が書く）で、
// ここでは書き換えない。off の課題にレビューを足すのは差分から機械が決めたレベルだけ。
func ReviewRequired(tier, review string) bool {
	return tier == "B" || tier == "C" || review == "on"
}

// NeedsReview は課題にレビューが要るかを、gate と同じ条件表・同じ分類で決める。
// review が on なら PR を読まずに要る。off なら PR の差分のレベル（受け入れ条件の manual と risk.one_way で上げる分を含む。
// review: on で上げる分は含まない）で決め、決めたレベルと理由を返す。on・off のどちらでもなければ要らない（呼ぶ側が断る）。
// 差分は今の PR のもので、呼ぶ側が PR の head（head）を確かめてから呼ぶ（gate は回すときに自分で読み直す）。
// 積んだ課題（l の Chief of Staff と rows で信じられる stacked_on・stacked_base がある）の差分は、gate と同じ規則（stackedFrom。
// PR が前の課題の branch に向いている間だけ分かれ目から）で数える（ADR-0067 決めたこと 9）。l は review が off のときだけ読む。
func NeedsReview(l *ledger.Ledger, rows []ledger.Row, t ledger.Issue, review string, rep *contract.Report, head string) (bool, Classified, error) {
	repo := t.Str("repo")
	switch review {
	case "on":
		return true, Classified{}, nil
	case "off":
	default:
		return false, Classified{}, nil
	}
	if rep == nil {
		return false, Classified{}, errors.New("report.json が無い")
	}
	owner, name, number, ok := contract.ParsePR(rep.PR)
	if !ok {
		return false, Classified{}, errors.New("report.json に pr が無い・形が違う")
	}
	table, _, err := LoadTable(getenv("ANDON_HOME", "/opt/andon"), delegate.Workspace())
	if err != nil {
		return false, Classified{}, err
	}
	slug := owner + "/" + name
	// 積んだ課題だけ PR の向き先を読む（積んでいない課題は今までどおり gh を余分に呼ばない）
	from := ""
	if _, _, ok := ledger.Stacked(rows, t, l.ChiefOfStaff()); ok {
		out, err := gh("pr", "view", number, "--repo", slug, "--json", "baseRefName")
		if err != nil {
			return false, Classified{}, err
		}
		var v struct {
			BaseRefName string `json:"baseRefName"`
		}
		if err := json.Unmarshal(out, &v); err != nil || v.BaseRefName == "" {
			return false, Classified{}, fmt.Errorf("PR の向き先が読めない: %v", err)
		}
		from = stackedFrom(l, rows, t, v.BaseRefName)
	}
	files, diff, err := prChanges(from, t, slug, number, head)
	if err != nil {
		return false, Classified{}, err
	}
	// 知らない repo は空の表で分ける（gate はその repo を needs_human に倒すが、レビューが要るかは同じ条件で決める）
	cl, err := table.Classify(table.Repos[repo], files, string(diff), false)
	if err != nil {
		return false, Classified{}, err
	}
	cl, _ = FromReport(cl, rep)
	return ReviewRequired(cl.Tier, review), cl, nil
}

func TierLine(cl Classified) string {
	reasons := cl.TierReasons
	more := ""
	if len(reasons) > 3 {
		more = fmt.Sprintf("ほか %d 件", len(reasons)-3)
		reasons = reasons[:3]
	}
	if more != "" {
		reasons = append(append([]string{}, reasons...), more)
	}
	if len(reasons) == 0 {
		return "レベル " + cl.Tier
	}
	return fmt.Sprintf("レベル %s（%s）", cl.Tier, strings.Join(reasons, "・"))
}
