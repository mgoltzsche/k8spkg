package k8spkg

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/mgoltzsche/k8spkg/pkg/transform"
	"github.com/pkg/errors"
)

type K8sPackage struct {
	*PackageInfo
	Objects []*model.K8sObject
}

func PkgFromManifest(reader io.Reader, namespace, name string) (pkg *K8sPackage, err error) {
	var namespaces []string
	if namespace == "" {
		var obj []*model.K8sObject
		if obj, err = model.FromReader(reader); err != nil {
			return
		}
		namespaces = containedNamespaces(obj)
		reader = manifestReader(obj)
	} else {
		namespaces = []string{namespace}
	}
	reader = manifest2pkgobjects(reader, namespace, name, namespaces)
	obj, err := model.FromReader(reader)
	if err != nil {
		return nil, errors.Wrap(err, "manifest2pkg")
	}
	if len(obj) == 0 {
		return nil, errors.New("no objects found in the provided manifest")
	}
	pkgs, err := PackageInfosFromObjects(obj)
	if err != nil {
		return
	}
	if len(pkgs) != 1 {
		return nil, errors.Errorf("1 package expected but %d provided", len(pkgs))
	}
	return &K8sPackage{pkgs[0], obj}, err
}

func manifest2pkgobjects(reader io.Reader, namespace, name string, namespaces []string) io.Reader {
	if namespace == "" && name == "" && len(namespaces) == 0 {
		return reader
	}
	pReader, pWriter := io.Pipe()
	go func() {
		err := transform.Transform(pWriter, func(tmpDir string) (tOpt *transform.TransformOptions, err error) {
			var f *os.File
			if f, err = os.OpenFile(filepath.Join(tmpDir, "manifest.yaml"), os.O_CREATE|os.O_WRONLY, 0600); err == nil {
				_, err = io.Copy(f, reader)
			}
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

func containedNamespaces(obj []*model.K8sObject) (ns []string) {
	nsMap := map[string]bool{}
	for _, o := range obj {
		if o.Namespace != "" && !nsMap[o.Namespace] {
			nsMap[o.Namespace] = true
			ns = append(ns, o.Namespace)
		}
	}
	sort.Strings(ns)
	return
}
