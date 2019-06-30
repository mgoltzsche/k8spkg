package labels

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mgoltzsche/k8spkg/pkg/kustomize"
	"github.com/pkg/errors"
)

var render func(kustomize.RenderOptions) error = kustomize.Render

func AddLabels(manifest io.Reader, labels map[string]string, writer io.Writer) (err error) {
	defer func() {
		if err != nil {
			err = errors.Wrap(err, "add labels")
		}
	}()
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return
	}
	defer os.RemoveAll(dir)
	if err = writeKustomizationFile(dir, labels); err != nil {
		return
	}
	if err = writeManifestFile(dir, manifest); err != nil {
		return
	}
	return render(kustomize.RenderOptions{
		Source: dir,
		Out:    writer,
	})
}

func writeKustomizationFile(dir string, labels map[string]string) (err error) {
	f, err := os.OpenFile(filepath.Join(dir, "kustomization.yaml"), os.O_CREATE|os.O_WRONLY, 0600)
	if err == nil {
		defer f.Close()
		kustomization := "resources:\n  - manifest.yaml\n"
		kustomization += "commonLabels:"
		for k, v := range labels {
			kustomization += fmt.Sprintf("\n  %q: %q", k, v)
		}
		_, err = io.Copy(f, bytes.NewReader([]byte(kustomization)))
	}
	return
}

func writeManifestFile(dir string, manifest io.Reader) (err error) {
	f, err := os.OpenFile(filepath.Join(dir, "manifest.yaml"), os.O_CREATE|os.O_WRONLY, 0600)
	if err == nil {
		defer f.Close()
		_, err = io.Copy(f, manifest)
	}
	return
}
