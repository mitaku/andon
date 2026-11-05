package boot

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FilesHandler は root（納品物 artifacts/）を読み取りで返す。要求先の実体のパス（symlink を解いたもの）が
// root の中にあるものだけ返し、外を指す symlink は 404 にする。root 自体が symlink なら作らない。
func FilesHandler(root string) (http.Handler, error) {
	st, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s が実ディレクトリでない（symlink は配らない）", root)
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	fsrv := http.FileServer(http.Dir(real))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "読み取りだけ", http.StatusMethodNotAllowed)
			return
		}
		target, err := filepath.EvalSymlinks(filepath.Join(real, filepath.FromSlash(filepath.Clean("/"+r.URL.Path))))
		if err != nil || (target != real && !strings.HasPrefix(target, real+string(filepath.Separator))) {
			http.NotFound(w, r)
			return
		}
		fsrv.ServeHTTP(w, r)
	}), nil
}

// serveFiles は andon boot serve-files <dir> <port>。boot が files サーバとして自分を起こすときの形。
func serveFiles(dir, port string) error {
	h, err := FilesHandler(dir)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: "127.0.0.1:" + port, Handler: h, ReadHeaderTimeout: 10 * time.Second}
	return srv.ListenAndServe()
}
