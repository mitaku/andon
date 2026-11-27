package issue

import "io"

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	leaves := map[string]func([]string, io.Reader, io.Writer, io.Writer) int{
		"show": Show, "report": Report, "review-request": ReviewRequest, "ready": Ready, "ask": Ask, "candidates": Candidates,
		"record": Record, "finish": Finish, "settle": Settle, "merge": Merge, "impact": Impact, "start": Start, "resume": Resume,
		"session": Session, "stop-check": StopCheck, "restack": Restack,
	}
	if len(args) == 0 || leaves[args[0]] == nil {
		return exitUsage
	}
	return leaves[args[0]](args[1:], stdin, stdout, stderr)
}
