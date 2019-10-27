package k8spkg

import (
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/mgoltzsche/k8spkg/pkg/transform"
	"github.com/pkg/errors"
)

// K8sPackage define a collection of objects and their package metadata
type K8sPackage struct {
	Name      string
	Resources resource.K8sResourceList
}

func PkgFromManifest(reader io.Reader, namespace, name string) (pkg *K8sPackage, err error) {
	obj, err := transformedObjects(reader, namespace, name)
	if err != nil {
		return
	}
	if name == "" {
		for _, o := range obj {
			labels := o.Labels()
			oldName := name
			name = labels[PKG_NAME_LABEL]
			if name == "" {
				return nil, errors.New("no package name provided")
			}
			if oldName != "" && oldName != name {
				return nil, errors.Errorf("resources contain different values for package name label %s", PKG_NAME_LABEL)
			}
		}
	}
	return &K8sPackage{name, obj}, nil
}

// transformedObjects read API objects from reader and modify their name and namespace if provided
func transformedObjects(reader io.Reader, namespace, name string) (obj resource.K8sResourceList, err error) {
	if obj, err = resource.FromReader(reader); err != nil {
		return
	}
	readCloser := obj.YamlReader()
	reader = readCloser
	defer readCloser.Close()
	transformed := manifest2pkgobjects(reader, name, namespace)
	defer transformed.Close()
	obj, err = resource.FromReader(transformed)
	if err != nil {
		return nil, errors.Wrap(err, "manifest2pkg")
	}
	if len(obj) == 0 {
		return nil, errors.New("no objects found in the provided manifest")
	}
	return
}

func manifest2pkgobjects(reader io.Reader, name, namespace string) io.ReadCloser {
	if namespace == "" && name == "" {
		return &noopReadCloser{reader}
	}
	pReader, pWriter := io.Pipe()
	go func() {
		err := transform.Transform(pWriter, func(tmpDir string) (tOpt *transform.TransformOptions, err error) {
			f, err := os.OpenFile(filepath.Join(tmpDir, "manifest.yaml"), os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return
			}
			defer f.Close()
			_, err = io.Copy(f, reader)
			labels := map[string]string{
				"app.kubernetes.io/managed-by": "k8spkg",
			}
			if name != "" {
				labels[PKG_NAME_LABEL] = name
			}
			if namespace != "" {
				labels[PKG_NS_LABEL] = namespace
			}
			return &transform.TransformOptions{
				Resources:    []string{"manifest.yaml"},
				Namespace:    namespace,
				CommonLabels: labels,
			}, err
		})
		pWriter.CloseWithError(err)
	}()
	return pReader
}

func containedNamespaces(obj []*resource.K8sResource) (n []string) {
	nsMap := map[string]bool{}
	for _, o := range obj {
		ns := o.Namespace()
		if ns != "" && !nsMap[ns] {
			nsMap[ns] = true
			n = append(n, ns)
		}
	}
	sort.Strings(n)
	return
}

type noopReadCloser struct {
	io.Reader
}

func (r *noopReadCloser) Close() error {
	return nil // do nothing
}
