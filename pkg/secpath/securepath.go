package secpath

import (
	"fmt"
	"path/filepath"
)

func SecurePath(file, rootDir string) (err error) {
	if file, err = filepath.EvalSymlinks(file); err != nil {
		return
	}
	if rootDir, err = filepath.EvalSymlinks(rootDir); err != nil {
		return
	}
	if !filepath.HasPrefix(file, rootDir) {
		err = fmt.Errorf("path %s is outside project directory %s", file, rootDir)
	}
	return
}
