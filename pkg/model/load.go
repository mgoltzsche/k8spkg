package model

import (
	"context"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"github.com/hashicorp/go-getter"
	urlhelper "github.com/hashicorp/go-getter/helper/url"
	"github.com/pkg/errors"
)

// Provides a reader for the given manifest dir or URL.
func ManifestReader(ctx context.Context, src, baseDir string) (reader io.ReadCloser) {
	reader, writer := io.Pipe()
	go func() {
		writer.CloseWithError(copySourceFiles(ctx, src, baseDir, writer))
	}()
	return
}

func copySourceFiles(ctx context.Context, src, baseDir string, writer io.Writer) (err error) {
	dSrc, err := getter.Detect(src, baseDir, getter.Detectors)
	if err != nil {
		return
	}
	var u *url.URL
	u, err = urlhelper.Parse(dSrc)
	if err != nil {
		return
	}
	file := u.Path
	if u.RawPath != "" {
		file = u.RawPath
	}
	if u.Scheme != "file" {
		g := getter.Getters[u.Scheme]
		if g == nil {
			return errors.Errorf("no getter mapped for URL scheme %q", u.Scheme)
		}
		var mode getter.ClientMode
		if mode, err = g.ClientMode(u); err != nil {
			return
		}
		if file, err = ioutil.TempDir("", "k8spkg-"); err != nil {
			return
		}
		defer os.RemoveAll(file)
		os.RemoveAll(file)
		c := &getter.Client{
			Dst:  file,
			Src:  dSrc,
			Ctx:  ctx,
			Mode: mode,
		}
		if err = c.Get(); err != nil {
			return
		}
	}
	err = copyFiles(ctx, file, writer)
	return errors.Wrapf(err, "source %s", src)
}

func copyFiles(ctx context.Context, file string, writer io.Writer) (err error) {
	si, err := os.Stat(file)
	if err == nil {
		if si.IsDir() {
			err = copyManifestDir(ctx, file, writer)
		} else {
			err = copyManifestFile(file, writer)
		}
	}
	return
}

func copyManifestDir(ctx context.Context, dir string, writer io.Writer) (err error) {
	var files []string
	extensions := []string{".yaml", ".yml", ".json"}
	for _, fext := range extensions {
		matches, e := filepath.Glob(filepath.Join(dir, "*"+fext))
		if e != nil {
			return e
		}
		files = append(files, matches...)
	}
	if len(files) == 0 {
		return errors.Errorf("no manifest files found within dir %s, recognized file extensions are %+v", dir, extensions)
	}
	sort.Strings(files)
	for _, file := range files {
		writer.Write([]byte("\n---\n"))
		if err = copyManifestFile(file, writer); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

func copyManifestFile(file string, writer io.Writer) (err error) {
	f, err := os.Open(file)
	if err == nil {
		defer f.Close()
		_, err = io.Copy(writer, f)
	}
	return
}
