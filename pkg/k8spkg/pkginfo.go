package k8spkg

import (
	"sort"
	"strings"

	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type PackageInfo struct {
	Name       string
	Namespaces []string
}

type pkgInfo struct {
	name                 string
	namespaces           map[string]bool
	namespaceLabelNotSet bool
}

// Returns a list of the packages declared within the provided objects.
// An error is only returned if an object has no k8spkg label while all
// declared packages are still returned.
func PackageInfosFromObjects(obj []*model.K8sObject) (pkgs []*PackageInfo, err error) {
	pkgMap := map[string]*pkgInfo{}
	var pkg *pkgInfo
	for _, o := range obj {
		labels := o.Labels()
		packageName := labels[PKG_NAME_LABEL]
		namespacesStr := labels[PKG_NS_LABEL]
		if packageName == "" {
			err = errors.Errorf("%s declares no package name label %s", o.ID(), PKG_NAME_LABEL)
			continue
		}
		if pkg = pkgMap[packageName]; pkg == nil {
			pkg = &pkgInfo{packageName, map[string]bool{}, false}
			pkgMap[packageName] = pkg
		}
		if o.Namespace == "" && namespacesStr == "" {
			pkg.namespaceLabelNotSet = true
		}
		if namespacesStr != "" {
			for _, ns := range strings.Split(namespacesStr, ".") {
				if ns != "" {
					pkg.namespaces[ns] = true
				}
			}
		}
		if o.Namespace != "" {
			pkg.namespaces[o.Namespace] = true
		}
	}
	pkgNames := make([]string, 0, len(pkgMap))
	for pkgName, _ := range pkgMap {
		pkgNames = append(pkgNames, pkgName)
	}
	sort.Strings(pkgNames)
	for _, pkgName := range pkgNames {
		pkg := pkgMap[pkgName]
		ns := make([]string, 0, len(pkg.namespaces))
		for nsName, _ := range pkg.namespaces {
			ns = append(ns, nsName)
		}
		sort.Strings(ns)
		if pkg.namespaceLabelNotSet {
			logrus.Warnf("package %s has cluster-scoped API objects without namespace label %s but namespaced objects as well (ns label required to retrieve all objects that belong to the package)", pkgName, PKG_NS_LABEL)
		}
		pkgs = append(pkgs, &PackageInfo{pkg.name, ns})
	}
	return
}
