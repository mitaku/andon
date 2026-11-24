package otel

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"andon/internal/delegate"
)

// Limits は生の記録の寿命と上限（決めたこと 10）。
type Limits struct {
	Days     int   // 最後に書いてからこの日数を過ぎたファイルを消す
	MaxBytes int64 // プロジェクトの置き場の合計がこれを超えたら、古いファイルから消す
}

// Default は既定の寿命と上限（30 日・10 GB）。project.json の otel で替える（LimitsOf。決めたこと 11）。
var Default = Limits{Days: 30, MaxBytes: 10 << 30}

// LimitsOf は project.json の otel（無ければ nil）から寿命と上限を決める。無いキーは既定のまま。
// 送らない設定（enabled が false）でも片付けは回す（前に書いた記録を残し続けない）。
func LimitsOf(o *delegate.Otel) Limits {
	lim := Default
	if o == nil {
		return lim
	}
	if o.Days != nil {
		lim.Days = *o.Days
	}
	if o.MaxGB != nil {
		lim.MaxBytes = int64(*o.MaxGB) << 30
	}
	return lim
}

// pruneEvery は片付けを走らせる間（1 日に 1 回）。
const pruneEvery = 24 * time.Hour

// stampName は前に片付けた時刻の印（プロジェクトの置き場の直下。mtime が時刻。.jsonl でないので片付けは消さない）。
const stampName = ".pruned"

type rawFile struct {
	path string
	size int64
	mod  time.Time
}

// Prune は dir（プロジェクトの置き場）の生の記録（*.jsonl）を片付け、消したファイルの数を返す。
// 最後に書いてから lim.Days 日を過ぎたものを消し、残りの合計が lim.MaxBytes を超えていれば古いものから消す。
// 中身の無くなった課題と役のフォルダも消す。消すだけなので、2 度走っても結果は同じ。失敗はまとめて返し、
// 1 つの失敗で残りを止めない。dir が無ければ何もしない。
func Prune(dir string, now time.Time, lim Limits) (int, error) {
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	var files []rawFile
	var dirs []string
	var errs error
	walkErr := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			errs = errors.Join(errs, err)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != dir {
				dirs = append(dirs, p)
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil // symlink は辿らず消さない
		}
		info, err := d.Info()
		if err != nil {
			errs = errors.Join(errs, err)
			return nil
		}
		files = append(files, rawFile{path: p, size: info.Size(), mod: info.ModTime()})
		return nil
	})
	errs = errors.Join(errs, walkErr)
	removed := 0
	remove := func(f rawFile) bool {
		if err := os.Remove(f.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = errors.Join(errs, err)
			return false
		}
		removed++
		return true
	}
	cutoff := now.Add(-time.Duration(lim.Days) * 24 * time.Hour)
	var kept []rawFile
	var total int64
	for _, f := range files {
		if f.mod.Before(cutoff) && remove(f) {
			continue
		}
		kept = append(kept, f)
		total += f.size
	}
	if total > lim.MaxBytes {
		sort.Slice(kept, func(i, j int) bool {
			if !kept[i].mod.Equal(kept[j].mod) {
				return kept[i].mod.Before(kept[j].mod)
			}
			return kept[i].path < kept[j].path
		})
		for _, f := range kept {
			if total <= lim.MaxBytes {
				break
			}
			if remove(f) {
				total -= f.size
			}
		}
	}
	// 深い順に、中身の無いフォルダを消す（中身があれば Remove が断るので、そのまま残す）
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, d := range dirs {
		_ = os.Remove(d) // 中身のあるフォルダは消えないのが正しい
	}
	return removed, errs
}

// PruneDue は、前の片付けから 1 日たっていれば、プロジェクトの置き場を limits が返す寿命と上限で片付ける
// （dispatcher が 1 巡ごとに呼ぶ。dispatcher.md §2.14・§7.2 #30）。limits は走らせるときだけ呼ぶ（project.json を読むのは 1 日に 1 回）。
// 走らせたら ran が true。
// 置き場が無いプロジェクトは何もしない。片付けが失敗しても印は書く（失敗を 1 分ごとに繰り返さず、次の日に走らせ直す）。
func PruneDue(project string, now time.Time, limits func() Limits) (ran bool, removed int, err error) {
	if !projectRe.MatchString(project) {
		return false, 0, errors.New("プロジェクトの名前の形が違う: " + project)
	}
	dir := filepath.Join(Root(), project)
	if st, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return false, 0, nil
	} else if err != nil {
		return false, 0, err
	} else if !st.IsDir() {
		return false, 0, errors.New("生の記録の置き場がフォルダでない: " + dir)
	}
	stamp := filepath.Join(dir, stampName)
	if st, err := os.Stat(stamp); err == nil && now.Sub(st.ModTime()) < pruneEvery && !st.ModTime().After(now) {
		return false, 0, nil
	}
	removed, err = Prune(dir, now, limits())
	if werr := touch(stamp, now); werr != nil {
		err = errors.Join(err, werr)
	}
	return true, removed, err
}

func touch(path string, at time.Time) error {
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return err
	}
	return os.Chtimes(path, at, at)
}
