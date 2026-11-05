package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// kanban writes only inside .flightdeck/. Task ids and the id prefix become folder names,
// so they must be one plain path segment; and nothing on a write path may be a symbolic
// link, which could point outside.

// checkSegment refuses a name that isn't exactly one path segment.
func checkSegment(what, s string) error {
	if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\:\x00") || filepath.VolumeName(s) != "" || filepath.IsAbs(s) {
		return fail("%s %q must be one plain name (no /, \\, :, . or ..)", what, s)
	}
	return nil
}

// checkNoLink refuses an existing path that is a symbolic link (or, with wantFile, not a regular file).
// A path that doesn't exist yet is fine.
func checkNoLink(path string, wantFile bool) error {
	fi, err := os.Lstat(path)
	switch {
	case err != nil:
		if os.IsNotExist(err) {
			return nil
		}
		return err
	case fi.Mode()&fs.ModeSymlink != 0:
		return fail("%s is a symbolic link; kanban writes only inside .flightdeck/", path)
	case wantFile && !fi.Mode().IsRegular():
		return fail("%s is not a regular file", path)
	}
	return nil
}

// taskPath is tasks/<id>, after checking that id is one segment and that neither
// tasks/ nor tasks/<id> is a symbolic link.
func taskPath(root, id string) (string, error) {
	if err := checkSegment("task id", id); err != nil {
		return "", err
	}
	dir := taskDir(root, id)
	if rel, err := filepath.Rel(tasksDir(root), dir); err != nil || rel != id {
		return "", fail("task id %q is outside .flightdeck/tasks/", id)
	}
	for _, p := range []string{tasksDir(root), dir} {
		if err := checkNoLink(p, false); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// appendFile appends to a regular file (made if missing), never through a symbolic link.
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
