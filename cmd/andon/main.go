// andon は建屋の仕事の入口（docs/design.md §2.2・§2.7）。中身は internal/ にある。
package main

import (
	"os"

	"andon/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
// MEASURE: Go を変えた PR（最後に戻す）
