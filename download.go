package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

func downloadURL(url, dest string) (err error) {
	destDir := filepath.Dir(dest)
	if err = os.MkdirAll(destDir, 0755); err != nil {
		return
	}
	out, err := ioutil.TempFile(destDir, ".tmp-*")
	if err != nil {
		return
	}
	defer func() {
		tmpName := out.Name()
		if e := out.Close(); e != nil && err == nil {
			err = e
		}
		if err == nil {
			err = os.Rename(tmpName, dest)
		} else {
			os.Remove(tmpName)
		}
	}()
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer func() {
		if e := resp.Body.Close(); e != nil && err == nil {
			err = e
		}
	}()
	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return
	}
	if n <= 0 {
		return fmt.Errorf("0 bytes written")
	}
	return
}
