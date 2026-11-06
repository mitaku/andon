package boot

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// FilesHandler は root（納品物 files/）を読み取りで返す。開くのは os.Root を通すので、
// root の外を指す symlink（ディレクトリの index.html を含む）は開けず 404 になる。root 自体が symlink なら作らない。
func FilesHandler(root string) (http.Handler, error) {
	st, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s が実ディレクトリでない（symlink は配らない）", root)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	fsrv := http.FileServerFS(r.FS())
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			http.Error(w, "読み取りだけ", http.StatusMethodNotAllowed)
			return
		}
		fsrv.ServeHTTP(w, req)
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
