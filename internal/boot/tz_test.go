package boot

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tzFixture は zoneinfo（Asia/Tokyo と Etc/UTC）・/etc/localtime の置き場・引数をそのまま打つ sudo を偽で置く。
func tzFixture(t *testing.T, tz string) (e Env, f *fixture) {
	t.Helper()
	f = newFixture(t)
	zi := filepath.Join(f.root, "zoneinfo")
	put(t, filepath.Join(zi, "Asia", "Tokyo"), "TZif tokyo\n")
	put(t, filepath.Join(zi, "Etc", "UTC"), "TZif utc\n")
	lt := filepath.Join(f.root, "etc", "localtime")
	mustMkdir(t, filepath.Dir(lt))
	if err := os.Symlink(filepath.Join(zi, "Etc", "UTC"), lt); err != nil {
		t.Fatal(err)
	}
	sudo := filepath.Join(f.root, "sudo")
	put(t, sudo, "#!/bin/sh\n[ \"$1\" = -n ] && shift\nexec \"$@\"\n")
	if err := os.Chmod(sudo, 0o755); err != nil {
		t.Fatal(err)
	}
	e = f.env()
	e.TZ, e.Zoneinfo, e.Localtime = tz, zi, lt
	e.LookPath = func(n string) (string, error) {
		if n == "sudo" {
			return sudo, nil
		}
		return "", errors.New("無い")
	}
	return e, f
}

// TZ の zoneinfo に /etc/localtime を向ける。2度目は同じなので打たない（ログも出さない）。
func TestSetLocaltime(t *testing.T) {
	e, f := tzFixture(t, "Asia/Tokyo")
	if err := setLocaltime(e); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(e.Zoneinfo, "Asia", "Tokyo")
	if got, _ := os.Readlink(e.Localtime); got != want {
		t.Errorf("/etc/localtime の先: %q（want %q）", got, want)
	}
	if !strings.Contains(f.log.String(), "タイムゾーン: ") {
		t.Errorf("ログ: %q", f.log.String())
	}
	f.log.Reset()
	if err := setLocaltime(e); err != nil {
		t.Fatal(err)
	}
	if f.log.Len() != 0 {
		t.Errorf("同じ向きでも打った: %q", f.log.String())
	}
}

// TZ が空なら触らない。名前でない値・zoneinfo に無い名前は断り、/etc/localtime はそのまま。
func TestSetLocaltimeRefuses(t *testing.T) {
	for _, tz := range []string{"", "Asia/Nowhere", ":/etc/localtime", "/usr/share/zoneinfo/Asia/Tokyo", "../zoneinfo/Asia/Tokyo", "JST-9", "Asia"} {
		e, _ := tzFixture(t, tz)
		before, _ := os.Readlink(e.Localtime)
		err := setLocaltime(e)
		if (tz == "") != (err == nil) {
			t.Errorf("TZ=%q: err=%v", tz, err)
		}
		if after, _ := os.Readlink(e.Localtime); after != before {
			t.Errorf("TZ=%q: /etc/localtime が変わった: %q → %q", tz, before, after)
		}
	}
}
