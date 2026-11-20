// Package pkgversion は、パッケージの置き場（ANDON_HOME。既定 /opt/andon）がどの commit から来たかと、
// 中身がその commit と食い違うかを読む（ADR-0028 決めたこと 3・P0-1）。
//
// 印は2つ。どちらも置き場の直下にあり、中身には数えない:
//
//	version.json  焼いたときの印（image の build と image/apply-main.sh が andon version stamp で書く）。
//	              {"commit": <40桁か null>, "dirty": <焼いた checkout に commit していない変更があったか>, "digest": "sha256:…"}
//	applied.json  当て直しの記録（ADR-0027。image/apply-main.sh が書く）。あれば commit はこちらを読む
//
// digest は置き場の全部のファイル（記録の2つと python の __pycache__ を除く）の、パス・実行の印・中身の sha256 を
// パスの順に並べたものの sha256。ファイルごとのハッシュは残さない（ADR-0028 の捨てた案）。
package pkgversion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Usage は andon version の使い方。
const Usage = "usage: andon version [stamp <revision>]\n" +
	"パッケージの置き場（ANDON_HOME。既定 /opt/andon）の commit と、中身がその commit と食い違うかを JSON で出す（読むだけ）。\n" +
	"  root     置き場\n" +
	"  commit   当て直しの記録（applied.json）があればその commit、無ければ焼いたときの commit。分からなければ null\n" +
	"  source   commit をどこから読んだか（applied・image・none）\n" +
	"  drift    中身が commit と食い違うか。true なら当時の文を git から復元できない。確かめられなければ null\n" +
	"  reasons  drift が true か null のときの理由\n" +
	"stamp <revision> は焼くとき（image の build・image/apply-main.sh）だけ打つ。置き場の中身を数えて version.json を書く。\n" +
	"<revision> は 40 桁の commit（汚れた checkout なら末尾に -dirty）か unknown。\n"

// Info は andon version の出力。
type Info struct {
	Root    string   `json:"root"`
	Commit  *string  `json:"commit"`
	Source  string   `json:"source"`
	Drift   *bool    `json:"drift"`
	Reasons []string `json:"reasons"`
}

type stamp struct {
	Commit *string `json:"commit"`
	Dirty  bool    `json:"dirty"`
	Digest string  `json:"digest"`
}

const (
	stampFile   = "version.json"
	appliedFile = "applied.json"
)

var (
	commitRe   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	revisionRe = regexp.MustCompile(`^([0-9a-f]{40})(-dirty)?$`)
)

// Home はパッケージの置き場（ANDON_HOME。無ければ /opt/andon）。
func Home() string {
	if h := os.Getenv("ANDON_HOME"); h != "" {
		return h
	}
	return "/opt/andon"
}

// Stamp は root の中身を数え、revision（40 桁の commit〈末尾に -dirty も可〉か unknown）と一緒に version.json を書く。
func Stamp(root, revision string) error {
	s := stamp{}
	if revision != "unknown" {
		m := revisionRe.FindStringSubmatch(revision)
		if m == nil {
			return fmt.Errorf("revision %q は 40 桁の commit（末尾に -dirty も可）か unknown でない", revision)
		}
		c := m[1]
		s.Commit = &c
		s.Dirty = m[2] != ""
	}
	d, err := Digest(root)
	if err != nil {
		return err
	}
	s.Digest = d
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(root, stampFile+".new")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(root, stampFile))
}

// Digest は root の中身の digest（"sha256:<hex>"）。
func Digest(root string) (string, error) {
	var lines []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == stampFile || rel == appliedFile || rel == stampFile+".new" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			t, err := os.Readlink(p)
			if err != nil {
				return err
			}
			lines = append(lines, "l "+rel+"\x00"+t)
		case info.Mode().IsRegular():
			h, err := fileHash(p)
			if err != nil {
				return err
			}
			kind := "f"
			if info.Mode()&0o111 != 0 {
				kind = "x"
			}
			lines = append(lines, kind+" "+rel+"\x00"+h)
		default:
			lines = append(lines, "o "+rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	h := sha256.New()
	for _, l := range lines {
		h.Write([]byte(l + "\n")) // hash.Hash の Write は失敗しない（errcheck も既定で見ない）
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func fileHash(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // 読むだけのファイルで、閉じる失敗は読んだ中身に効かない
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Read は root の commit と食い違いを読む。読めないものは null と理由にし、エラーにしない。
func Read(root string) Info {
	info := Info{Root: root, Source: "none", Reasons: []string{}}
	reason := func(s string) { info.Reasons = append(info.Reasons, s) }

	var st *stamp
	if b, err := os.ReadFile(filepath.Join(root, stampFile)); err == nil {
		var s stamp
		if err := json.Unmarshal(b, &s); err != nil || !strings.HasPrefix(s.Digest, "sha256:") ||
			(s.Commit != nil && !commitRe.MatchString(*s.Commit)) {
			reason(stampFile + " が読めない（形が違う）")
		} else {
			st = &s
		}
	} else if !os.IsNotExist(err) {
		reason(stampFile + " が読めない: " + err.Error())
	}

	var appliedCommit *string
	if b, err := os.ReadFile(filepath.Join(root, appliedFile)); err == nil {
		var a struct {
			Commit string `json:"commit"`
		}
		if err := json.Unmarshal(b, &a); err != nil || !commitRe.MatchString(a.Commit) {
			reason(appliedFile + " が読めない（形が違う）")
		} else {
			appliedCommit = &a.Commit
		}
	} else if !os.IsNotExist(err) {
		reason(appliedFile + " が読めない: " + err.Error())
	}

	switch {
	case appliedCommit != nil:
		info.Commit, info.Source = appliedCommit, "applied"
	case st != nil:
		info.Commit, info.Source = st.Commit, "image"
	}

	if st == nil {
		if len(info.Reasons) == 0 {
			reason(stampFile + " が無い（焼いたときの印が無いので、中身を確かめられない）")
		}
		return info
	}
	drift := false
	if st.Commit == nil {
		drift = true
		reason("焼いた元の commit が分からない（unknown）")
	}
	if st.Dirty {
		drift = true
		reason("焼いた checkout に commit していない変更があった")
	}
	if appliedCommit != nil && (st.Commit == nil || *st.Commit != *appliedCommit) {
		drift = true
		reason(appliedFile + " の commit と " + stampFile + " の commit が違う")
	}
	d, err := Digest(root)
	switch {
	case err != nil:
		reason("中身を数えられない: " + err.Error())
		if !drift {
			info.Drift = nil
			return info
		}
	case d != st.Digest:
		drift = true
		reason("中身が焼いたときと違う（手で書き換えた・足した・消した）")
	}
	info.Drift = &drift
	return info
}

// Main は andon version。
func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	root := Home()
	switch {
	case len(args) == 0:
		b, err := json.MarshalIndent(Read(root), "", "  ")
		if err != nil {
			fmt.Fprintln(stderr, "✗", err)
			return 1
		}
		fmt.Fprintln(stdout, string(b))
		return 0
	case args[0] == "stamp" && len(args) == 2:
		if err := Stamp(root, args[1]); err != nil {
			fmt.Fprintln(stderr, "✗", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s/%s を書いた\n", root, stampFile)
		return 0
	}
	fmt.Fprint(stderr, Usage)
	return 2
}
