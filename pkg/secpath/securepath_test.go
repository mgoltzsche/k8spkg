package secpath

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestSecurePath(t *testing.T) {
	dir, err := ioutil.TempDir("", "tmp-test-securepath-")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	defer os.RemoveAll(dir)
	subdir := filepath.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	os.Symlink("..", filepath.Join(subdir, "badlink"))
	os.Symlink("/etc/passwd", filepath.Join(subdir, "badlink2"))

	for _, badPath := range []string{
		"..",
		"subdir/../..",
		"/etc/passwd",
		"subdir/badlink/subdir/badlink2",
	} {
		if err = SecurePath(filepath.Join(dir, badPath), dir); err == nil {
			t.Errorf("malicous path %q should raise error", badPath)
		}
	}

	for _, godPath := range []string{
		".",
		"subdir",
	} {
		if err = SecurePath(filepath.Join(dir, godPath), dir); err != nil {
			t.Errorf("unexpected error for path %q: %s", godPath, err)
		}
	}
}
