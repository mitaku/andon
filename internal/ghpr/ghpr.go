// Package ghpr は gh pr list で repo のマージされた PR を読む。
package ghpr

import (
	"encoding/json"
	"strconv"
)

// Limit は gh pr list に最初に渡す --limit。
const Limit = 1000

// Merged は gh pr list で repo（<owner>/<repo>）のマージされた PR を fields の欄で全部読み、gh の出力をそのまま返す。
// gh は新しい順に --limit 件で切るので、返った件数が --limit に届いたら、その外がまだあるとみて上限を倍にして読み直す。
// 出力が配列として読めなければ、そのまま返す（読めないことは呼び出し元が言う）。
func Merged(gh func(args ...string) ([]byte, error), repo, fields string) ([]byte, error) {
	for limit := Limit; ; limit *= 2 {
		b, err := gh("pr", "list", "--repo", repo, "--state", "merged", "--limit", strconv.Itoa(limit), "--json", fields)
		if err != nil {
			return nil, err
		}
		var l []json.RawMessage
		if json.Unmarshal(b, &l) != nil || len(l) < limit {
			return b, nil
		}
	}
}
