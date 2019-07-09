package transform

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mgoltzsche/k8spkg/pkg/kustomize"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var render func(kustomize.RenderOptions) error = kustomize.Render

type TransformOptions struct {
	Resources    []string          `yaml:"resources,omitempty"`
	Namespace    string            `yaml:"namespace,omitempty"`
	CommonLabels map[string]string `yaml:"commonLabels,omitempty"`
}

func (o *TransformOptions) Empty() bool {
	return o.Namespace == "" && len(o.Resources) == 0 && len(o.CommonLabels) == 0
}

type GeneratorFn func(tmpDir string) (*TransformOptions, error)

func Transform(writer io.Writer, generator GeneratorFn) (err error) {
	dir, err := ioutil.TempDir("", "")
	if err == nil {
		defer os.RemoveAll(dir)
		var opts *TransformOptions
		if opts, err = generator(dir); err == nil {
			if len(opts.Resources) == 0 {
				return errors.New("no resources provided with kustomization options")
			}
			if err = writeKustomizationFile(dir, opts); err == nil {
				err = render(kustomize.RenderOptions{
					Source: dir,
					Out:    writer,
				})
			}
		}
	}
	return errors.Wrap(err, "transform")
}

func writeKustomizationFile(dir string, kustomization *TransformOptions) (err error) {
	f, err := os.OpenFile(filepath.Join(dir, "kustomization.yaml"), os.O_CREATE|os.O_WRONLY, 0600)
	if err == nil {
		defer f.Close()
		var b []byte
		if b, err = yaml.Marshal(kustomization); err == nil {
			_, err = io.Copy(f, bytes.NewReader(b))
		}
	}
	return
}
