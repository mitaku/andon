// Package andon は repo の根にある資材を andon のバイナリに埋め込む（go:embed は自分より下しか読めないため、根に置く）。
package andon

import "embed"

// Init は建屋の repo の雛形（init/）。andon init が写す。
//
//go:embed all:init
var Init embed.FS
