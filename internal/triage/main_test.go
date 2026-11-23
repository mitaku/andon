package triage

import "io"

// Main は検査が葉を名前で打つ口（andon の振り分けは internal/cli の葉の表。ここは検査のためだけ）。
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	leaves := map[string]func([]string, io.Reader, io.Writer, io.Writer) int{
		"collect": Collect,
		"apply":   Apply,
		"verdict": Verdict,
	}
	if len(args) == 0 || leaves[args[0]] == nil {
		return 2
	}
	return leaves[args[0]](args[1:], stdin, stdout, stderr)
}
