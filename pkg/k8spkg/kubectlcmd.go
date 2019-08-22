package k8spkg

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type kubectlCmd struct {
	ctx            context.Context
	kubeconfigFile string
	Stdout         io.Writer
	Stderr         io.Writer
	Stdin          io.Reader
}

func newKubectlCmd(ctx context.Context, kubeconfigFile string) *kubectlCmd {
	return &kubectlCmd{
		ctx:            ctx,
		kubeconfigFile: kubeconfigFile,
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
	}
}

func (c *kubectlCmd) Run(args ...string) (err error) {
	if c.kubeconfigFile != "" {
		args = append([]string{"--kubeconfig", c.kubeconfigFile}, args...)
	}
	cmd := exec.CommandContext(c.ctx, "kubectl", args...)
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Stdin = c.Stdin
	logrus.Debugf("Running %+v", cmd.Args)
	err = cmd.Run()
	if err != nil && c.ctx.Err() != nil {
		return errors.WithStack(c.ctx.Err())
	}
	return errors.Wrapf(err, "%+v", cmd.Args)
}
