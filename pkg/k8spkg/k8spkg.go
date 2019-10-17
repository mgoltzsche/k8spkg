package k8spkg

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/mgoltzsche/k8spkg/pkg/transform"
	"github.com/pkg/errors"
)

// K8sPackage define a collection of objects and their package metadata
type K8sPackage struct {
	*PackageInfo
	Resources resource.K8sResourceList
}

// TransformedObjects read API objects from reader and modify their name and namespace if provided
func TransformedObjects(reader io.Reader, namespace, name string) (obj resource.K8sResourceList, err error) {
	var namespaces []string
	if namespace == "" {
		if obj, err = resource.FromYaml(reader); err != nil {
			return
		}
		namespaces = containedNamespaces(obj)
		readCloser := obj.YamlReader()
		reader = readCloser
		defer readCloser.Close()
	} else {
		namespaces = []string{namespace}
	}
	transformed := manifest2pkgobjects(reader, namespace, name, namespaces)
	defer transformed.Close()
	obj, err = resource.FromYaml(transformed)
	if err != nil {
		return nil, errors.Wrap(err, "manifest2pkg")
	}
	if len(obj) == 0 {
		return nil, errors.New("no objects found in the provided manifest")
	}
	return
}

func PkgFromManifest(reader io.Reader, namespace, name string) (pkg *K8sPackage, err error) {
	obj, err := TransformedObjects(reader, namespace, name)
	if err != nil {
		return
	}
	pkgs, err := PackageInfosFromResources(obj)
	if err != nil {
		return
	}
	if len(pkgs) != 1 {
		return nil, errors.Errorf("1 package expected but %d provided", len(pkgs))
	}
	return &K8sPackage{pkgs[0], obj}, nil
}

func manifest2pkgobjects(reader io.Reader, namespace, name string, namespaces []string) io.ReadCloser {
	if namespace == "" && name == "" && len(namespaces) == 0 {
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
			if len(namespaces) > 0 {
				labels[PKG_NS_LABEL] = strings.Join(namespaces, ".")
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
