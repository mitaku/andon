package otel

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

// ServeUsage は andon otel serve の使い方。
const ServeUsage = "usage: andon otel serve\n" +
	"ハーネス（Claude Code・Codex）の OTLP/HTTP の JSON のトレースとログを " + Addr + " で受け、\n" +
	"個人の属性を落として秘密の形を伏せてから ~/.local/state/andon/otel/<project>/ の課題ごと・役ごとのファイルに書く。\n" +
	"受けるのは POST /v1/traces と /v1/logs だけ。GET / は名前 " + Name + " を返す。\n" +
	"ポートをほかのプロセスが取っていれば、起動せずに終了コード 1（二重の起動は bind の失敗で防ぐ）。\n"

// ServeMain は andon otel serve。
func ServeMain(args []string, _ io.Reader, _, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprint(stderr, ServeUsage)
		return 2
	}
	ln, err := net.Listen("tcp", Addr)
	if err != nil {
		fmt.Fprintf(stderr, "andon otel serve: %s を取れない（ほかの受け口が動いているか）: %v\n", Addr, err)
		return 1
	}
	if err := serve(ln, &Receiver{Root: Root(), Log: stderr}); err != nil {
		fmt.Fprintf(stderr, "andon otel serve: %v\n", err)
		return 1
	}
	return 0
}

func serve(ln net.Listener, r *Receiver) error {
	if err := os.MkdirAll(r.Root, 0o700); err != nil {
		return err
	}
	srv := &http.Server{Handler: r, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: time.Minute}
	return srv.Serve(ln)
}
