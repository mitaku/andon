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

var subs = []string{"issue", "chief-of-staff", "create", "open", "chief-engineer", "dispatcher", "news", "publish", "triage", "signal", "measure", "process", "boot", "init", "gate", "delegate"}

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
	if rc != 2 || out != "" || !strings.Contains(errs, "知らない sub-command: bogus") || !strings.Contains(errs, "\n  dispatcher  ") {
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

func TestNoSkeletonLeft(t *testing.T) {
	for _, c := range commands {
		if c.run == nil {
			t.Errorf("%s の中身が無い", c.name)
		}
	}
}

func TestPassesArgsToSubCommand(t *testing.T) {
	if rc, _, errs := run("triage"); rc != 2 || !strings.HasPrefix(errs, "usage: andon triage collect") {
		t.Errorf("triage: rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := run("dispatcher", "start"); rc != 2 || !strings.HasPrefix(errs, "usage: andon dispatcher") {
		t.Errorf("dispatcher: rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := run("chief-of-staff", "start"); rc != 2 || !strings.HasPrefix(errs, "usage: andon chief-of-staff") {
		t.Errorf("chief-of-staff: rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := run("issue", "start", "X-1", "extra"); rc != 2 || !strings.HasPrefix(errs, "usage: andon issue") {
		t.Errorf("issue start: rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := run("chief-engineer"); rc != 2 || !strings.HasPrefix(errs, "usage: andon chief-engineer") {
		t.Errorf("chief-engineer: rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := run("publish", "/nonexistent/none.html"); rc != 1 || !strings.Contains(errs, "not a file") {
		t.Errorf("publish: rc=%d err=%q", rc, errs)
	}
}

// ADR-0008 決めたこと 7: andon task は別名を持たない。知らない command として止まり、一覧と issue の使い方に task が出ない。
func TestNoTaskCommand(t *testing.T) {
	for _, args := range [][]string{{"task"}, {"task", "start", "X-1"}, {"task", "--help"}} {
		if rc, out, errs := run(args...); rc != 2 || out != "" || !strings.Contains(errs, "知らない sub-command: task") {
			t.Errorf("%v: rc=%d out=%q err=%q", args, rc, out, errs)
		}
	}
	_, help, _ := run("help")
	_, issue, _ := run("issue", "--help")
	for name, s := range map[string]string{"help": help, "issue --help": issue} {
		if regexp.MustCompile(`(?i)\btask`).MatchString(s) {
			t.Errorf("%s に task が出る:\n%s", name, s)
		}
	}
}
