package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	defaultTimeout = time.Duration(2 * time.Minute)
)

type K8sClient interface {
	Apply(ctx context.Context, namespace string, resources resource.K8sResourceList, prune bool, labels []string) (resource.K8sResourceList, error)
	Delete(ctx context.Context, namespace string, resources resource.K8sResourceRefList) (err error)
	GetResource(ctx context.Context, kind string, namespace string, name string) (*resource.K8sResource, error)
	Get(ctx context.Context, kinds []string, namespace string, labels []string) <-chan resource.ResourceEvent
	//WatchResource(ctx context.Context, kind, namespace string, name string) <-chan WatchEvent
	Watch(ctx context.Context, kind, namespace string, labels []string, watchOnly bool) <-chan resource.ResourceEvent
	AwaitDeletion(ctx context.Context, namespace string, resources resource.K8sResourceRefList) (err error)
	ResourceTypes(ctx context.Context) (types []*APIResourceType, err error)
	ContainerLogs(ctx context.Context, namespace, podName, containerName string, previous, follow bool, writer io.Writer) (err error)
}

type notFoundError struct {
	error
}

func IsNotFound(err error) bool {
	_, ok := err.(notFoundError)
	return ok
}

// APIResourceType represents a Kubernetes API resource type's metadata
type APIResourceType struct {
	Name       string
	ShortNames []string
	APIGroup   string
	Kind       string
	Namespaced bool
}

// Returns the type's short name if any or its name
func (t *APIResourceType) ShortName() (name string) {
	name = t.Name
	if len(t.ShortNames) > 0 {
		name = t.ShortNames[0]
	}
	return
}

// Returns the type's short name with APIGroup suffix if there is one
func (t *APIResourceType) FullName() (name string) {
	if t.APIGroup == "" {
		return t.ShortName()
	}
	return t.ShortName() + "." + t.APIGroup
}

type k8sClient struct {
	kubeconfigFile string
}

type WatchEvent struct {
	Resource *resource.K8sResource
	Error    error
}

func NewK8sClient(kubeconfigFile string) K8sClient {
	return &k8sClient{kubeconfigFile}
}

func (c *k8sClient) Apply(ctx context.Context, namespace string, resources resource.K8sResourceList, prune bool, labelSelector []string) (l resource.K8sResourceList, err error) {
	args := []string{"apply", "--wait", "-f", "-", "--record", "--timeout=" + getTimeout(ctx)}
	if len(labelSelector) > 0 {
		args = append(args, "-l", strings.Join(labelSelector, ","))
	}
	if prune {
		// TODO: delete objects within other namespaces that belong to the package as well
		args = append(args, "--prune")
	}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	for evt := range c.kubectlEmit(ctx, resources.YamlReader(), args) {
		if evt.Error == nil {
			l = append(l, evt.Resource)
		} else {
			err = evt.Error
		}
	}
	return
}

func (c *k8sClient) Delete(ctx context.Context, namespace string, resources resource.K8sResourceRefList) (err error) {
	for _, grp := range resources.GroupByNamespace() {
		args := []string{"delete", "--wait", "--cascade", "--ignore-not-found", "--timeout=" + getTimeout(ctx)}
		args = append(args, grp.Resources.Names()...)
		if grp.Key == "" {
			grp.Key = namespace
		}
		if grp.Key != "" {
			args = append(args, "-n", grp.Key)
		}
		if e := kubectl(ctx, nil, nil, c.kubeconfigFile, args); e != nil && err == nil {
			err = e
		}
	}
	return
}

func (c *k8sClient) AwaitDeletion(ctx context.Context, namespace string, resources resource.K8sResourceRefList) error {
	for _, grp := range resources.GroupByNamespace() {
		args := []string{"wait", "--for", "delete", "--timeout=" + getTimeout(ctx)}
		args = append(args, grp.Resources.Names()...)
		if grp.Key == "" {
			grp.Key = namespace
		}
		if grp.Key != "" {
			args = append(args, "-n", grp.Key)
		}
		if err := kubectl(ctx, nil, nil, c.kubeconfigFile, args); err != nil {
			if kerr, ok := errors.Cause(err).(*kubectlError); ok {
				var unexpectedLines []string
				for _, line := range kerr.stderr {
					if !strings.HasPrefix(line, "Error from server (NotFound): ") {
						unexpectedLines = append(unexpectedLines, line)
					}
				}
				if len(unexpectedLines) > 0 {
					return &kubectlError{kerr.error, unexpectedLines}
				}
			} else {
				return err
			}
		}
	}
	return ctx.Err()
}

func (c *k8sClient) GetResource(ctx context.Context, kind string, namespace string, name string) (r *resource.K8sResource, err error) {
	args := []string{"--ignore-not-found", strings.ToLower(kind), name}
	for evt := range c.kubectlEmit(ctx, nil, getArgs(namespace, args...)) {
		if evt.Error != nil && err == nil {
			err = evt.Error
			continue
		}
		r = evt.Resource
	}
	if r == nil && err == nil {
		err = notFoundError{errors.Errorf("resource %s:%s/%s not found", namespace, kind, name)}
	}
	return
}

func (c *k8sClient) Get(ctx context.Context, kinds []string, namespace string, labels []string) <-chan resource.ResourceEvent {
	args := []string{strings.ToLower(strings.Join(kinds, ","))}
	if len(labels) > 0 {
		args = append(args, "-l", strings.Join(labels, ","))
	}
	return c.kubectlEmit(ctx, nil, getArgs(namespace, args...))
}

/*func (c *k8sClient) WatchResource(ctx context.Context, kind, namespace string, name string) <-chan resource.ResourceEvent {
	return c.kubectlGet(ctx, namespace, []string{"-w", strings.ToLower(kind), name})
}*/

func (c *k8sClient) Watch(ctx context.Context, kind, namespace string, labels []string, watchOnly bool) <-chan resource.ResourceEvent {
	args := []string{"-w", strings.ToLower(kind)}
	if len(labels) > 0 {
		args = append(args, "-l", strings.Join(labels, ","))
	}
	if watchOnly {
		args = append(args, "--watch-only")
	}
	return c.kubectlEmit(ctx, nil, getArgs(namespace, args...))
}

func (c *k8sClient) ContainerLogs(ctx context.Context, namespace, podName, containerName string, previous, follow bool, writer io.Writer) (err error) {
	args := []string{"logs", podName, "-c", containerName}
	if previous {
		args = append(args, "--previous")
	} else if follow {
		args = append(args, "--follow")
	}

	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	return kubectl(ctx, nil, writer, c.kubeconfigFile, args)
}

func (c *k8sClient) kubectlEmit(ctx context.Context, stdin io.Reader, args []string) <-chan resource.ResourceEvent {
	reader, writer := io.Pipe()
	done := make(chan error)
	ch := make(chan resource.ResourceEvent)
	go func() {
		var err error
		for evt := range resource.FromJsonStream(reader) {
			if evt.Error != nil && err == nil {
				err = evt.Error
			} else {
				ch <- evt
			}
		}
		reader.CloseWithError(err)
		done <- err
	}()
	go func() {
		args = append(args, "-o", "json")
		err := kubectl(ctx, stdin, writer, c.kubeconfigFile, args)
		writer.CloseWithError(err)
		if e := <-done; e != nil && err == nil {
			err = errors.Wrap(e, "get")
		}
		if err != nil {
			ch <- resource.ResourceEvent{nil, err}
		}
		close(ch)
	}()
	return ch
}

func kubectl(ctx context.Context, in io.Reader, out io.Writer, kubeconfigFile string, args []string) (err error) {
	if kubeconfigFile != "" {
		args = append(args, "--kubeconfig", kubeconfigFile)
	}
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = &buf
	logrus.Debugf("Running %+v", cmd.Args)
	err = cmd.Run()
	if err != nil && ctx.Err() != nil {
		return errors.WithStack(ctx.Err())
	}
	stderr := buf.String()
	if err != nil && len(stderr) > 0 {
		stderr = strings.TrimSpace(stderr)
		err = &kubectlError{errors.WithStack(err), strings.Split(stderr, "\n")}
	}
	if e := ctx.Err(); e != nil && err == nil {
		err = e
	} else {
		err = errors.Wrapf(err, "%+v", cmd.Args)
	}
	return
}

type kubectlError struct {
	error
	stderr []string
}

func (e *kubectlError) Error() string {
	return fmt.Sprintf("%s. stderr: %s", e.error, strings.Join(e.stderr, "\n  "))
}

func getArgs(namespace string, args ...string) []string {
	args = append([]string{"get"}, args...)
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	return args
}

func getTimeout(ctx context.Context) string {
	t, ok := ctx.Deadline()
	if ok {
		return t.Sub(time.Now().Add(time.Second)).String()
	}
	return defaultTimeout.String()
}
