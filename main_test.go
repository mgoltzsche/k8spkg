package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mgoltzsche/k8spkg/pkg/k8spkg"
	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifest(t *testing.T) {
	srv := httptest.NewServer(http.FileServer(http.Dir(".")))
	addr := srv.Listener.Addr().String()
	defer srv.Close()

	for _, c := range []struct {
		expectedPkgName string
		expectedCount   int
		args            []string
	}{
		{"somepkg", 8, []string{"manifest", "-f", "pkg/model/test"}},
		{"withname", 8, []string{"manifest", "-f", "pkg/model/test", "--name", "withname"}},
		{"kustomizedpkg", 2, []string{"manifest", "-k", "pkg/model/test/kustomize"}},
		{"remoteFile", 2, []string{"manifest", "-f", "http://" + addr + "/pkg/model/test/manifestdir/some-cert.yaml", "--name", "remoteFile"}},
	} {
		out := testRun(t, c.args)
		obj, err := model.FromReader(bytes.NewReader(out))
		require.NoError(t, err, "FromReader(%s)", c.expectedPkgName)
		require.Equal(t, c.expectedCount, len(obj), "%s object count", c.expectedPkgName)
		pkgName := ""
		for _, o := range obj {
			if pkgName = o.Labels()[k8spkg.PKG_NAME_LABEL]; pkgName != c.expectedPkgName {
				break
			}
		}
		assert.Equal(t, c.expectedPkgName, pkgName, "package name")
	}
}

func testRun(t *testing.T, args []string) []byte {
	var buf bytes.Buffer
	err := run(append([]string{"k8spkg"}, args...), &buf)
	require.NoError(t, err, "%+v", args)
	return buf.Bytes()
}
