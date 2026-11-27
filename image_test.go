package andon

import (
	"os"
	"regexp"
	"testing"
)

// image/Dockerfile の段 tray（ローカルの tray をビルドする oven/bun。ADR-0064 決めたこと 26）の bun のバージョンは mise.toml の bun と揃える。
// tests/ の下のシェルは Bun を名指せない（TestWorkflowTrayOnlyInTrayJob）ので、ここで見る。
func TestImageBunMatchesMise(t *testing.T) {
	read := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if msg := bunMismatch(read("mise.toml"), read("image/Dockerfile")); msg != "" {
		t.Error(msg)
	}
	// 当たる場合があること: バージョンを違えた写しと、段の無い写しは落ちる
	for name, df := range map[string]string{
		"違う": "FROM oven/bun:0.0.1-debian@sha256:00 AS tray\n",
		"無い": "FROM ubuntu:24.04\n",
	} {
		if bunMismatch("bun = \"1.4.5\"\n", df) == "" {
			t.Errorf("%s: バージョンが揃わない写しを通した", name)
		}
	}
}

var (
	miseBun  = regexp.MustCompile(`(?m)^bun = "([^"]+)"$`)
	imageBun = regexp.MustCompile(`(?m)^FROM oven/bun:([0-9.]+)-\S+ AS tray$`)
)

func bunMismatch(mise, dockerfile string) string {
	m, d := miseBun.FindStringSubmatch(mise), imageBun.FindStringSubmatch(dockerfile)
	switch {
	case m == nil:
		return "mise.toml に bun のバージョンが無い"
	case d == nil:
		return "image/Dockerfile に段 tray（FROM oven/bun:<バージョン>-… AS tray）が無い"
	case m[1] != d[1]:
		return "image/Dockerfile の oven/bun（" + d[1] + "）が mise.toml の bun（" + m[1] + "）と違う"
	}
	return ""
}
