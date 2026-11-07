package spawn

import (
	"os"
	"testing"
)

func TestCharDevicesAreNotTerminals(t *testing.T) {
	for _, p := range []string{os.DevNull, "/dev/zero"} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		if IsTerminal(f) {
			t.Errorf("%s を端末とみなした", p)
		}
		f.Close()
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if IsTerminal(r) {
		t.Error("パイプを端末とみなした")
	}
}
