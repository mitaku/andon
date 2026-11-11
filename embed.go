// Package andon は repo の根にある資材を andon のバイナリに埋め込む（go:embed は自分より下しか読めないため、根に置く）。
package andon

import "embed"

// Init はセルの repo の雛形（init/）。andon init が写す。
//
//go:embed all:init
var Init embed.FS

// IssueStates は課題の状態の定義（process/states/issue.yaml。ADR-0017 決めたこと 15）。andon issue … が、
// 定義に無い移り方を断るのに読む。
//
//go:embed process/states/issue.yaml
var IssueStates []byte
