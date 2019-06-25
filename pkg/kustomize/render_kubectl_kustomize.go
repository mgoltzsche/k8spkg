// +build kubectl_kustomize

package kustomize

import (
	"bytes"
	"os/exec"

	"github.com/pkg/errors"
)

func Render(o RenderOptions) (err error) {
	var stderr bytes.Buffer
	c := exec.Command("kubectl", "kustomize", o.Source)
	c.Stdout = o.Out
	c.Stderr = &stderr
	if err = c.Run(); err != nil {
		err = errors.Errorf("%+v: %s. %s", c.Args, err, stderr.String())
	}
	return
}
