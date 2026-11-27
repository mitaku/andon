package tray_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/tray"
	"andon/internal/tray/traytest"
)

// keyAt は検査の間だけ、Cloudflare の tray の API キーの置き場を一時の場所にし、key を置く（空なら置かない）。
func keyAt(t *testing.T, key string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if key == "" {
		return
	}
	if err := os.MkdirAll(tray.KeyDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tray.KeyPath(), []byte(key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// RemoteFor は project.json の tray のキーを、オーナーの commit のときだけ読む（ADR-0064 決めたこと 5・14）。
func TestRemoteForReadsProjectJSON(t *testing.T) {
	keyAt(t, "tray_key000000000000")
	ws := traytest.CellRepo(t, traytest.Owner, map[string]string{
		"chosen": traytest.ProjectJSON("https://tray-up.example.com/"),
		"plain":  traytest.ProjectJSON(""),
	})
	r, err := tray.RemoteFor(ws, "chosen")
	if err != nil || r == nil || r.Upload != "https://tray-up.example.com" || r.Key != "tray_key000000000000" {
		t.Fatalf("選んだプロジェクト: %+v %v", r, err)
	}
	for _, p := range []string{"plain", "absent", "", tray.NoProject} {
		if r, err := tray.RemoteFor(ws, p); err != nil || r != nil {
			t.Errorf("%q は選んでいない: %+v %v", p, r, err)
		}
	}
	// 鍵が無くても選んだことは返す（送る段が鍵の無いことを理由にする）
	keyAt(t, "")
	if r, err := tray.RemoteFor(ws, "chosen"); err != nil || r == nil || r.Key != "" {
		t.Errorf("鍵の無い選んだプロジェクト: %+v %v", r, err)
	}
}

// 信じられない（オーナーでない author の）project.json と、読めないセルの repo は誤りで、選んだことにしない（安全側）。
func TestRemoteForRefusesUntrusted(t *testing.T) {
	keyAt(t, "tray_key000000000000")
	ws := traytest.CellRepo(t, "someone@example.com", map[string]string{"chosen": traytest.ProjectJSON("https://tray-up.example.com")})
	if r, err := tray.RemoteFor(ws, "chosen"); err == nil || r != nil {
		t.Errorf("オーナーでない commit の tray を読んだ: %+v %v", r, err)
	}
	if r, err := tray.RemoteFor(filepath.Join(t.TempDir(), "none"), "chosen"); err == nil || r != nil {
		t.Errorf("読めないセルの repo を「選んでいない」にした: %+v %v", r, err)
	}
	bad := traytest.CellRepo(t, traytest.Owner, map[string]string{"chosen": traytest.ProjectJSON("http://tray-up.example.com")})
	if _, err := tray.RemoteFor(bad, "chosen"); err == nil || !strings.Contains(err.Error(), "tray.upload") {
		t.Errorf("https でない tray.upload: %v", err)
	}
}
