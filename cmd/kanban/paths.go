package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func checkSegment(what, s string) error {
	if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\:\x00") || filepath.VolumeName(s) != "" || filepath.IsAbs(s) {
		return fail("%s %q must be one plain name (no /, \\, :, . or ..)", what, s)
	}
	return nil
}

func checkNoLink(path string, wantFile bool) error {
	fi, err := os.Lstat(path)
	switch {
	case err != nil:
		if os.IsNotExist(err) {
			return nil
		}
		return err
	case fi.Mode()&fs.ModeSymlink != 0:
		return fail("%s is a symbolic link; kanban writes only inside kanban/", path)
	case wantFile && !fi.Mode().IsRegular():
		return fail("%s is not a regular file", path)
	}
	return nil
}

func taskPath(root, id string) (string, error) {
	if err := checkSegment("issue id", id); err != nil {
		return "", err
	}
	dir := taskDir(root, id)
	if rel, err := filepath.Rel(issuesDir(root), dir); err != nil || rel != id {
		return "", fail("issue id %q is outside kanban/issues/", id)
	}
	for _, p := range []string{issuesDir(root), dir} {
		if err := checkNoLink(p, false); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func appendFile(path string, data []byte) error {
	if err := checkNoLink(path, true); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
