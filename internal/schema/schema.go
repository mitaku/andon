// Package schema は JSON を約束の schema と照らす口（andon schema）。検査器は command が書く前に照らすのと同じ
// internal/contract の Checker（既製の google/jsonschema-go と拡張の uniqueKeys）で、tests/schema.sh・tests/process.sh と、
// セルで JSON を手で確かめるのがこれを使う（ADR-0034 決めたこと 6 の 2 段目）。
package schema

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"andon/internal/contract"
)

const (
	CheckUsage = "usage: andon schema check <schema> <json>\n" +
		"<schema> は andon に埋め込んだ schema/ の直下の名前（report・question・findings など）か、*.schema.json のパス。\n" +
		"<json> は JSON のファイルのパスか -（標準入力）。\n" +
		"合えば終了コード 0。合わなければ 1 と理由（標準出力）。検査できなければ 2 と理由（標準エラー）。\n" +
		"検査できないのは、schema が読めない・検査器の知らないキーワードがある・JSON でない・置き場が読めないとき。\n"
	BatchUsage = "usage: andon schema batch <cases.tsv>\n" +
		"1行1件 `<schema><TAB><json>` を、検査器を1度読むだけでまとめて照らす（tests/schema.sh が使う）。\n" +
		"行ごとに `終了コード<TAB>理由の1行目` を出す（終了コードは check と同じ）。batch そのものは、表が読めれば 0。\n"
)

func CheckMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprint(stderr, CheckUsage)
		return 2
	}
	rc, why := check(args[0], args[1], stdin)
	switch rc {
	case 1:
		fmt.Fprintln(stdout, why)
	case 2:
		fmt.Fprintln(stderr, "andon schema: 検査できない: "+why)
	}
	return rc
}

func BatchMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprint(stderr, BatchUsage)
		return 2
	}
	return batch(args[0], stdin, stdout, stderr)
}

func isPath(arg string) bool { return strings.Contains(arg, "/") || strings.HasSuffix(arg, ".json") }

func names() string {
	var out []string
	for _, s := range contract.Schemas {
		out = append(out, string(s))
	}
	return strings.Join(out, "・")
}

// check は1件を照らし、(終了コード, 理由) を返す。0 合う・1 合わない・2 検査できない。
func check(schemaArg, jsonArg string, stdin io.Reader) (int, string) {
	var b []byte
	var err error
	if jsonArg == "-" {
		b, err = io.ReadAll(stdin)
	} else {
		b, err = os.ReadFile(jsonArg)
	}
	if err != nil {
		return 2, fmt.Sprintf("JSON が読めない: %v", err)
	}
	if isPath(schemaArg) {
		var c *contract.Checker
		if c, err = contract.CompileFile(schemaArg); err == nil {
			err = c.Check(b)
		}
	} else if s, ok := contract.ParseSchema(schemaArg); ok {
		err = s.Check(b)
	} else {
		return 2, fmt.Sprintf("埋め込んだ schema に %s は無い（%s）", schemaArg, names())
	}
	var mismatch *contract.MismatchError
	switch {
	case err == nil:
		return 0, ""
	case errors.As(err, &mismatch):
		return 1, mismatch.Error()
	default:
		return 2, err.Error()
	}
}

func batch(path string, stdin io.Reader, stdout, stderr io.Writer) int {
	r := stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(stderr, "andon schema: 表が読めない: %v\n", err)
			return 2
		}
		defer f.Close() //nolint:errcheck // 読むだけの表で、閉じる失敗は読んだ中身に効かない
		r = f
	}
	w := bufio.NewWriter(stdout)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		rc, why := 2, "batch の行の形が違う（<schema><TAB><json>）"
		if f := strings.Split(line, "\t"); len(f) == 2 && f[1] != "-" {
			rc, why = check(f[0], f[1], nil)
		}
		why, _, _ = strings.Cut(why, "\n")
		fmt.Fprintf(w, "%d\t%s\n", rc, strings.ReplaceAll(why, "\t", " "))
	}
	// 結果を書き切れなければ、呼び手が件数の食い違いでなく失敗として受けるよう 2 で終える
	if err := w.Flush(); err != nil {
		fmt.Fprintf(stderr, "andon schema: 結果を書けない: %v\n", err)
		return 2
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(stderr, "andon schema: 表が読めない: %v\n", err)
		return 2
	}
	return 0
}
