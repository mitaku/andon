package boot

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestHomeVolumesMatchCompose(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "package", "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range regexp.MustCompile(`(?m)^\s*-\s*[a-z-]+:/opt/andon-home/(\S+)\s*$`).FindAllStringSubmatch(string(b), -1) {
		got = append(got, m[1])
	}
	want := append([]string(nil), HomeVolumes...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("compose.yaml の volume %v と HomeVolumes %v が違う", got, want)
	}
}

func TestFixOwnersChownsOtherOwner(t *testing.T) {
	f := newFixture(t)
	for _, d := range []string{".ssh", ".config/gh"} {
		mustMkdir(t, filepath.Join(f.home, d))
	}
	calls := filepath.Join(f.root, "sudo-calls")
	sudo := filepath.Join(f.root, "bin", "sudo")
	put(t, sudo, "#!/bin/sh\necho \"$*\" >> '"+calls+"'\n")
	if err := os.Chmod(sudo, 0o755); err != nil {
		t.Fatal(err)
	}
	e := f.env()
	e.LookPath = func(string) (string, error) { return sudo, nil }
	e.Uid, e.Gid = os.Getuid()+1, os.Getgid()+1
	if err := fixOwners(e); err != nil {
		t.Fatal(err)
	}
	owner := strconv.Itoa(e.Uid) + ":" + strconv.Itoa(e.Gid)
	want := "-n chown -R " + owner + " " + filepath.Join(f.home, ".config/gh") + "\n" +
		"-n chown -R " + owner + " " + filepath.Join(f.home, ".ssh") + "\n"
	if got := read(t, calls); got != want {
		t.Errorf("sudo の呼び出し:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(f.log.String(), "置き場のオーナーを直した: "+filepath.Join(f.home, ".ssh")) {
		t.Errorf("ログ: %s", f.log.String())
	}

	os.Remove(calls)
	e.Uid, e.Gid = os.Getuid(), os.Getgid()
	if err := fixOwners(e); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(calls); err == nil {
		t.Errorf("置き場のオーナーが同じなのに sudo を打った: %s", read(t, calls))
	}
}
