package model

/*import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	if os.Args[0] == "kubectl" {
		// mock `kubectl kustomize`
		if len(os.Args) == 3 && os.Args[1] == "kustomize" {
			fmt.Println(`---
apiVersion: certmanager.k8s.io/v1alpha1
kind: SomeKind
metadata:
  name: kustomized1
  namespace: cert-manager
spec:
  duration: 23000h
---
apiVersion: certmanager.k8s.io/v1alpha1
kind: SomeKind
metadata:
  name: kustomized02
  namespace: cert-manager
spec:
  duration: 23000h`)
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "KUBECTLMOCK: unexpected kustomize call: %+v\n", os.Args)
		os.Exit(1)
	}
}

func TestObjects(t *testing.T) {
	tmpBin, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(tmpBin)
	err = os.Symlink("/proc/self/exe", filepath.Join(tmpBin, "kubectl"))
	require.NoError(t, err)
	os.Setenv("PATH", tmpBin+string(filepath.ListSeparator)+os.Getenv("PATH"))

	for _, c := range []struct {
		source        string
		expectedNames []string
	}{
		{"test/manifestdir/some-cert.yaml", []string{"somecert", "acert"}},
		{"test/manifestdir", []string{"some-ca", "somecert", "acert"}},
	} {
		ol, err := Objects(c.source, "")
		require.NoError(t, err, "Objects(%s)", c.source)
		names := []string{}
		for _, o := range ol {
			names = append(names, o.Metadata().Name)
		}
		assert.Equal(t, c.expectedNames, names, "source %s", c.source)
	}
}
*/
