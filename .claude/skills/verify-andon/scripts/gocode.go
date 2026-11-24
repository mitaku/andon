// gocode.go: Go のファイルのうち、コードの字句を変えたものだけを出す。
//
//	go run gocode.go <repo> <前の commit> <後の commit> < パス（1行1つ）
//
// パスごとに、前と後の中身を go/scanner で字句に分け、コメントを除いた並びを比べる。並びが違うパスを1行1つで出す。
// 振る舞いを変えるコメント（//go: の指令・//line・// +build・//export・//extern）は字句として残す。
// import "C" を持つファイル（cgo の前置きがコメントに入る）は、前と後のどちらかにあればコメントを全部残す。
// 片方に無いファイル・読めないファイル・字句に分けられないファイルは、変えたものとして出す（名指しを求める側に倒す）。
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"strings"
)

type lex struct {
	tok token.Token
	lit string
}

func show(repo, rev, path string) ([]byte, bool) {
	out, err := exec.Command("git", "-C", repo, "show", rev+":"+path).Output()
	return out, err == nil
}

// lexes は src の字句を返す。keepAll ならコメントを全部残す。字句に分けられなければ false。
func lexes(src []byte, keepAll bool) ([]lex, bool) {
	fset := token.NewFileSet()
	f := fset.AddFile("x.go", -1, len(src))
	bad := false
	var s scanner.Scanner
	s.Init(f, src, func(token.Position, string) { bad = true }, scanner.ScanComments)
	var got []lex
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		switch {
		case tok == token.COMMENT:
			if !keepAll && !directive(lit) {
				continue
			}
		case tok == token.SEMICOLON:
			lit = "" // 改行で入る ; と書いた ; は同じ
		}
		got = append(got, lex{tok, lit})
	}
	return got, !bad
}

func directive(c string) bool {
	for _, p := range []string{"//go:", "//line ", "/*line ", "//export ", "//extern "} {
		if strings.HasPrefix(c, p) {
			return true
		}
	}
	// go/build は // のあとの空白を除いてから +build を読む（//  +build も //\t+build も効く）
	return strings.HasPrefix(c, "//") && strings.HasPrefix(strings.TrimSpace(c[2:]), "+build")
}

// cgo は import "C" を持つか（字句の並びで見る。コメントや文字列の中の語には当たらない）。
func cgo(src []byte) bool {
	all, _ := lexes(src, false)
	for i, l := range all {
		if l.tok == token.STRING && (l.lit == `"C"` || l.lit == "`C`") {
			for j := i - 1; j >= 0; j-- {
				if all[j].tok == token.IMPORT {
					return true
				}
				if all[j].tok != token.LPAREN && all[j].tok != token.SEMICOLON && all[j].tok != token.STRING && all[j].tok != token.IDENT && all[j].tok != token.PERIOD {
					break
				}
			}
		}
	}
	return false
}

func codeChanged(a, b []byte) bool {
	keep := cgo(a) || cgo(b)
	la, oka := lexes(a, keep)
	lb, okb := lexes(b, keep)
	if !oka || !okb || len(la) != len(lb) {
		return true
	}
	for i := range la {
		if la[i] != lb[i] {
			return true
		}
	}
	return false
}

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "使い方: go run gocode.go <repo> <前の commit> <後の commit> < パス（1行1つ）")
		os.Exit(2)
	}
	repo, before, after := os.Args[1], os.Args[2], os.Args[3]
	in := bufio.NewScanner(os.Stdin)
	for in.Scan() {
		p := in.Text()
		if p == "" {
			continue
		}
		a, oka := show(repo, before, p)
		b, okb := show(repo, after, p)
		if !oka || !okb || !bytes.HasSuffix([]byte(p), []byte(".go")) || codeChanged(a, b) {
			fmt.Println(p)
		}
	}
	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "gocode:", err)
		os.Exit(1)
	}
}
