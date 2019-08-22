package k8spkg

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPkgFromManifest(t *testing.T) {
	// test plain manifest
	plainManifest := `---
apiVersion: certmanager.k8s.io/v1alpha1
kind: Issuer
metadata:
  name: ca-issuer
  namespace: cert-manager
---
apiVersion: certmanager.k8s.io/v1alpha1
kind: RoleBinding
metadata:
  name: ca-issuer
  namespace: kube-system
`
	pkg, err := PkgFromManifest(bytes.NewReader([]byte(plainManifest)), "myns", "somepkg")
	require.NoError(t, err)
	for _, o := range pkg.Objects {
		require.Equal(t, "somepkg", o.Labels()[PKG_NAME_LABEL], "pkg name")
		require.Equal(t, "myns", o.Namespace, "pkg namespace")
		require.Equal(t, "myns", o.Labels()[PKG_NS_LABEL], "pkg namespaces")
	}
	pkg, err = PkgFromManifest(bytes.NewReader([]byte(plainManifest)), "", "somepkg")
	require.NoError(t, err)
	require.Equal(t, 2, len(pkg.Objects), "len(pkg.Objects)")
	for _, o := range pkg.Objects {
		require.True(t, o.Namespace == "cert-manager" || o.Namespace == "kube-system", "unexpected namespace: "+o.Namespace)
		require.Equal(t, "cert-manager.kube-system", o.Labels()[PKG_NS_LABEL], "pkg namespace label")
	}
	pkg, err = PkgFromManifest(bytes.NewReader([]byte(plainManifest)), "", "")
	require.Error(t, err, "unlabeled package objects should yield error")

	// test k8spkg manifest
	pkgManifest := `---
apiVersion: certmanager.k8s.io/v1alpha1
kind: Issuer
metadata:
    name: ca-issuer
    namespace: cert-manager
    labels:
        ` + PKG_NAME_LABEL + `: somepkg
---
apiVersion: certmanager.k8s.io/v1alpha1
kind: RoleBinding
metadata:
    name: ca-issuer
    namespace: kube-system
    labels:
        ` + PKG_NAME_LABEL + `: somepkg
`
	pkg, err = PkgFromManifest(bytes.NewReader([]byte(pkgManifest)), "", "")
	require.NoError(t, err)
	require.Equal(t, 2, len(pkg.Objects), "len(pkg.Objects)")
	for _, o := range pkg.Objects {
		require.Equal(t, "somepkg", o.Labels()[PKG_NAME_LABEL], "pkg name label should be preserved")
		require.True(t, o.Namespace == "cert-manager" || o.Namespace == "kube-system", "unexpected namespace: "+o.Namespace)
		require.Equal(t, "cert-manager.kube-system", o.Labels()[PKG_NS_LABEL], "pkg namespace label in otherwise untouched manifest")
	}
}
