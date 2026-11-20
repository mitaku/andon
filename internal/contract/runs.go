package contract

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// StaleRun は、報告の受け入れ条件の行が指す記録のうち、head と違う commit で撮ったもの（gate の記録の acceptance.stale_runs）。
type StaleRun struct {
	ID     string `json:"id"`     // 受け入れ条件の行の id
	Run    int    `json:"run"`    // 記録の番号
	Commit string `json:"commit"` // 記録の commit
}

// CheckRuns は、report.json の acceptance[] の行のうち記録（library/runs/<n>.json）を指すものを確かめる
// （contract.md §24。ADR-0035 決めたこと 2・11）。library は課題の library/。
// 断る理由（problems）は3通り: 記録が無い・読めない（出力のファイルが無いのも含む）／終了コードが記録の期待と違う／
// pass を期待する記録が commit していない変更のある木で撮られている。fail を期待する記録は変更があってよい（決めたこと 8）。
// head が空でなければ、記録の commit が head と違う行を stale に返す（断る理由にはしない。gate が記録に出す）。
// 記録を指さない行（CI の検査の名前・添付の URL・manual）は見ない。
func CheckRuns(library string, acceptance []Acceptance, head string) (problems []string, stale []StaleRun) {
	for _, a := range acceptance {
		if a.Run == nil {
			continue
		}
		n := *a.Run
		name := fmt.Sprintf("受け入れ条件 %s の記録 runs/%d.json", a.ID, n)
		r, err := ReadRun(filepath.Join(library, "runs", strconv.Itoa(n)+".json"))
		switch {
		case errors.Is(err, os.ErrNotExist):
			problems = append(problems, name+" が無い")
			continue
		case err != nil:
			problems = append(problems, fmt.Sprintf("%s が読めない: %v", name, err))
			continue
		case r.ID != n:
			problems = append(problems, fmt.Sprintf("%s が読めない: id が %d", name, r.ID))
			continue
		}
		if _, err := os.Stat(filepath.Join(library, r.Output)); err != nil {
			problems = append(problems, fmt.Sprintf("%s の出力 %s が無い", name, r.Output))
		}
		if !r.Met() {
			problems = append(problems, fmt.Sprintf("%s の終了コード %d が期待（%s）と違う", name, r.ExitCode, r.Expect))
		}
		if r.Expect == "pass" && r.Dirty != nil && *r.Dirty {
			problems = append(problems, name+" は commit していない変更のある木で撮った（pass を期待する記録は commit してから撮り直す）")
		}
		if head != "" && r.Commit != "" && r.Commit != head {
			stale = append(stale, StaleRun{ID: a.ID, Run: n, Commit: r.Commit})
		}
	}
	return problems, stale
}
