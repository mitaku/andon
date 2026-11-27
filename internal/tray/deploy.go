package tray

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const DeployUsage = "usage: andon tray deploy --config <設定のファイル> [--dry-run]\n" +
	"Cloudflare の tray（3つの Worker・D1・R2）をオーナーの Cloudflare のアカウントへ出す（ADR-0064 決めたこと 16・27）。\n" +
	"セルの中では打たない。オーナーがセルを動かしている PC で、bin/bootstrap tray-deploy から andon のイメージで打つ。\n" +
	"Cloudflare のトークンは、この1回の実行の環境変数 CLOUDFLARE_API_TOKEN にだけ渡す。\n" +
	"設定のファイル（セルの repo の tray-deploy.json）は、アカウントの ID・3つのホスト名・Access のチームと aud を持つ（形は docs/operate.md）。\n" +
	"中身は tray/scripts/deploy.mjs（wrangler）を Node で動かす。何度打ってもよい。--dry-run は Cloudflare に触れず、組んだ設定を wrangler に確かめさせるだけ。\n"

// deployCmd は deploy.mjs を起動する口（検査が差し替える）。
var deployCmd = func(node string, args []string, stdout, stderr io.Writer) error {
	cmd := exec.Command(node, args...)
	cmd.Dir = Dir()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// lookNode は node を探す口（検査が差し替える）。
var lookNode = func() (string, error) { return exec.LookPath("node") }

func DeployMain(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	config, dry, err := deployArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "andon tray deploy: %v\n", err)
		fmt.Fprint(stderr, DeployUsage)
		return 2
	}
	if !dry && os.Getenv("CLOUDFLARE_API_TOKEN") == "" {
		fmt.Fprintln(stderr, "andon tray deploy: CLOUDFLARE_API_TOKEN が無い（この1回の実行にだけ渡す。セルの中には置かない。docs/operate.md の Cloudflare の tray の手順）")
		return 2
	}
	abs, err := filepath.Abs(config)
	if err != nil {
		fmt.Fprintf(stderr, "andon tray deploy: %v\n", err)
		return 2
	}
	if _, err := os.Stat(abs); err != nil {
		fmt.Fprintf(stderr, "andon tray deploy: 設定のファイル %s が読めない: %v\n", abs, err)
		return 2
	}
	node, err := lookNode()
	if err != nil {
		fmt.Fprintln(stderr, "andon tray deploy: node が無い")
		return 1
	}
	script := filepath.Join(Dir(), "scripts", "deploy.mjs")
	if _, err := os.Stat(script); err != nil {
		fmt.Fprintf(stderr, "andon tray deploy: %s が無い（イメージの tray をビルドしていない）\n", script)
		return 1
	}
	argv := []string{script, "--config", abs}
	if dry {
		argv = append(argv, "--dry-run")
	}
	if err := deployCmd(node, argv, stdout, stderr); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() > 0 {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "andon tray deploy: %v\n", err)
		return 1
	}
	return 0
}

func deployArgs(args []string) (config string, dry bool, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) || args[i+1] == "" {
				return "", false, errors.New("--config に設定のファイルが無い")
			}
			i++
			config = args[i]
		case "--dry-run":
			dry = true
		default:
			return "", false, fmt.Errorf("知らない引数 %q", args[i])
		}
	}
	if config == "" {
		return "", false, errors.New("--config が無い")
	}
	return config, dry, nil
}
