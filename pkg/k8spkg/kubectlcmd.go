package k8spkg

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type kubectlCmd struct {
	ctx            context.Context
	kubeconfigFile string
	Stdout         io.Writer
	Stdin          io.Reader
}

func newKubectlCmd(ctx context.Context, kubeconfigFile string) *kubectlCmd {
	return &kubectlCmd{
		ctx:            ctx,
		kubeconfigFile: kubeconfigFile,
		Stdout:         os.Stdout,
	}
}

func (c *kubectlCmd) Run(args ...string) (err error) {
	if c.kubeconfigFile != "" {
		args = append([]string{"--kubeconfig", c.kubeconfigFile}, args...)
	}
	var buf bytes.Buffer
	cmd := exec.CommandContext(c.ctx, "kubectl", args...)
	cmd.Stdin = c.Stdin
	cmd.Stdout = c.Stdout
	cmd.Stderr = &buf
	logrus.Debugf("Running %+v", cmd.Args)
	err = cmd.Run()
	if err != nil && c.ctx.Err() != nil {
		return errors.WithStack(c.ctx.Err())
	}
	stderr := buf.String()
	if err != nil && len(stderr) > 0 {
		err = errors.Errorf("%s, stderr: %s", err, strings.ReplaceAll(stderr, "\n", "\n  "))
	}
	return errors.Wrapf(err, "%+v", cmd.Args)
}
