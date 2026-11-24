package delegate

import "io"

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	leaves := map[string]func([]string, io.Reader, io.Writer, io.Writer) int{
		"grant":  GrantMain,
		"narrow": NarrowMain,
		"revoke": RevokeMain,
		"show":   ShowMain,
	}
	if len(args) == 0 || leaves[args[0]] == nil {
		return 2
	}
	return leaves[args[0]](args[1:], stdin, stdout, stderr)
}
