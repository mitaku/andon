package cli

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := Run(args, strings.NewReader(""), &out, &errb)
	return rc, out.String(), errb.String()
}

var subs = []string{"task", "chief-of-staff", "chief-engineer", "courier", "publish", "triage", "gate"}

func TestHelpListsEverySubCommand(t *testing.T) {
	rc, out, errs := run("help")
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	for _, s := range append(subs, "help") {
		if !regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(s) + `  `).MatchString(out) {
			t.Errorf("help に %s の行が無い:\n%s", s, out)
		}
	}
	for _, args := range [][]string{nil, {"-h"}, {"--help"}} {
		if rc2, out2, _ := run(args...); rc2 != 0 || out2 != out {
			t.Errorf("%v: rc=%d, help と違う", args, rc2)
		}
	}
}

func TestUnknownSubCommand(t *testing.T) {
	rc, out, errs := run("bogus")
	if rc != 2 || out != "" || !strings.Contains(errs, "知らない sub-command: bogus") || !strings.Contains(errs, "\n  courier  ") {
		t.Errorf("rc=%d out=%q err=%q", rc, out, errs)
	}
}

func TestSubCommandHelp(t *testing.T) {
	for _, s := range subs {
		rc, out, errs := run(s, "--help")
		if rc != 0 || errs != "" || !strings.HasPrefix(out, "usage: andon "+s) {
			t.Errorf("%s --help: rc=%d out=%q err=%q", s, rc, out, errs)
		}
	}
}

func TestSkeletonSubCommands(t *testing.T) {
	for _, s := range []string{"task", "chief-of-staff", "courier", "gate"} {
		rc, out, errs := run(s, "start", "X-1")
		if rc != 1 || out != "" || !strings.Contains(errs, "中身はまだ無い") {
			t.Errorf("%s: rc=%d out=%q err=%q", s, rc, out, errs)
		}
	}
}

func TestPassesArgsToSubCommand(t *testing.T) {
	if rc, _, errs := run("triage"); rc != 2 || !strings.HasPrefix(errs, "usage: andon triage collect") {
		t.Errorf("triage: rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := run("chief-engineer"); rc != 2 || !strings.HasPrefix(errs, "usage: andon chief-engineer") {
		t.Errorf("chief-engineer: rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := run("publish", "/nonexistent/none.html"); rc != 1 || !strings.Contains(errs, "not a file") {
		t.Errorf("publish: rc=%d err=%q", rc, errs)
	}
}
